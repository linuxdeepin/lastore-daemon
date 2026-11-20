"""Guard regression tests, including real dpkg operating only under /tmp."""
import importlib.util
import importlib.machinery
import os
import shlex
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
HOOK = HERE.parents[1] / 'usr' / 'libexec' / 'lastore-daemon' / 'dpkg-package-guard'
loader = importlib.machinery.SourceFileLoader('dpkg_guard', str(HOOK))
spec = importlib.util.spec_from_loader('dpkg_guard', loader)
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)
CORE = 'guard-test-desktop'


class GuardTest(unittest.TestCase):
    def test_arguments(self):
        with patch.dict(os.environ, {'DPKG_ADMINDIR': '/env/db'}):
            self.assertEqual(guard.operation_context(['dpkg', '-iR', '--root=/tmp/root',
                '--admindir', '/tmp/db', '--log', '/tmp/log', '--', 'a b'], True),
                ('/tmp/db', False, True, ['a b']))
            self.assertEqual(guard.removal_context(['dpkg', '-Pa']), ('/env/db', True, []))
            self.assertEqual(guard.operation_context(['dpkg', '--admindir=/tmp/db',
                '--root', '/tmp/root', '--instdir=/other', '-i', 'a.deb'], True)[0],
                '/tmp/root/var/lib/dpkg')
        with self.assertRaises(ValueError):
            guard.operation_context(['dpkg', '-i', '--unknown', 'a.deb'], True)
        with self.assertRaises(ValueError):
            guard.operation_context(['dpkg', '--root'], True)

    def test_removals(self):
        self.assertEqual(guard.protected_removals('remove', ['dpkg', '-r', CORE + ':amd64'], {CORE}), {CORE})
        with patch.object(guard, 'pending_removals', return_value={CORE}) as query:
            self.assertEqual(guard.protected_removals('purge', ['dpkg', '-Pa'], {CORE}), {CORE})
            query.assert_called_once()

    def test_metadata_and_failures(self):
        with patch.object(guard, 'checked_output', return_value='Package: example\nArchitecture: all\nConflicts: '+CORE+',\n other\n'):
            self.assertEqual(len(guard.conflict_relations(guard.control_fields('a')['conflicts'])), 2)
        for value in (CORE + ' | other', CORE + ' (invalid)', CORE + ','):
            with self.assertRaises(ValueError):
                guard.conflict_relations(value)
        for action in ('install', 'unpack'):
            with patch.dict(os.environ, {'DPKG_HOOK_ACTION': action}), \
                 patch.object(guard, 'load_protected_packages', return_value={CORE}), \
                 patch.object(guard, 'find_dpkg_argv', return_value=['dpkg', '-i', 'missing.deb']), \
                 patch.object(guard, 'installation_conflicts', side_effect=subprocess.TimeoutExpired('dpkg-deb', 15)):
                self.assertEqual(guard.main(), 1)
        with patch.object(guard, 'checked_output', return_value=f'{CORE}\tall\t1.0\tunpacked\n'):
            self.assertEqual(guard.installed_protected('/tmp/db', {CORE}), [(CORE, 'all', '1.0')])
        with patch.object(guard, 'checked_output', return_value=f'{CORE}\tall\t1.0\tconfig-files\n'):
            self.assertEqual(guard.installed_protected('/tmp/db', {CORE}), [])


class RealDpkgTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='dpkg-guard-test-')
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.root = self.directory / 'root'
        db = self.root / 'var/lib/dpkg'
        db.mkdir(parents=True)
        (db / 'status').touch()
        self.base = ['dpkg', '--root=' + str(self.root), '--force-not-root',
                     '--force-script-chrootless', '--log=' + str(self.directory / 'dpkg.log')]
        self.install(self.build(CORE))
        self.config = self.directory / 'guard.conf'
        self.packages = self.directory / 'protected-packages.list'
        self.config.write_text('[guard]\nenabled = true\n')
        self.packages.write_text(CORE + '\n')
        wrapper = self.directory / 'hook.py'
        wrapper.write_text(
            'import importlib.machinery\n'
            'import importlib.util\n'
            f'loader = importlib.machinery.SourceFileLoader("guard", {str(HOOK)!r})\n'
            'spec = importlib.util.spec_from_loader("guard", loader)\n'
            'guard = importlib.util.module_from_spec(spec)\n'
            'spec.loader.exec_module(guard)\n'
            f'raise SystemExit(guard.main({str(self.config)!r}, {str(self.packages)!r}))\n')
        self.hook = '--pre-invoke=' + shlex.join(['/usr/bin/python3', str(wrapper)])

    def build(self, package, relations='', version='1.0'):
        directory = self.directory / (package + '-' + version)
        (directory / 'DEBIAN').mkdir(parents=True, exist_ok=True)
        (directory / 'DEBIAN/control').write_text(
            f'Package: {package}\nVersion: {version}\nArchitecture: all\n'
            'Maintainer: test <test@example.invalid>\nDescription: isolated test\n' + relations)
        payload = directory / 'usr/share' / package / 'marker'
        payload.parent.mkdir(parents=True, exist_ok=True)
        payload.write_text('isolated test\n')
        deb = directory.with_suffix('.deb')
        subprocess.run(['dpkg-deb', '--build', str(directory), str(deb)], check=True, capture_output=True)
        return deb

    def install(self, deb):
        result = subprocess.run(self.base + ['--install', str(deb)], text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def run_hook(self, args):
        return subprocess.run(self.base + [self.hook] + list(map(str, args)), text=True, capture_output=True)

    def assert_core(self):
        result = subprocess.run(['dpkg-query', '--admindir=' + str(self.root / 'var/lib/dpkg'),
            '-W', '-f=${db:Status-Status}', CORE], text=True, capture_output=True, check=True)
        self.assertEqual(result.stdout, 'installed')
        self.assertTrue((self.root / 'usr/share' / CORE / 'marker').exists())

    def test_unprotected_baseline_reproduces_replacement(self):
        deb = self.build('replacement', f'Conflicts: {CORE}\nReplaces: {CORE}\n')
        self.install(deb)
        self.assertFalse((self.root / 'usr/share' / CORE / 'marker').exists())
        self.assertTrue((self.root / 'usr/share/replacement/marker').exists())

    def test_pending_and_corrupt_archive(self):
        for action, selection in (('--remove', 'deinstall'), ('--purge', 'purge')):
            result = subprocess.run(self.base + ['--set-selections'],
                input=CORE + ' ' + selection + '\n', text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            result = self.run_hook([action, '--pending'])
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('dpkg-package-guard: refusing', result.stderr)
            self.assert_core()
        bad = self.directory / 'corrupt.deb'
        bad.write_text('not a deb')
        with self.assertRaises(subprocess.CalledProcessError):
            guard.installation_conflicts(['dpkg', '--root=' + str(self.root), '-i', str(bad)], {CORE})

    def test_replacement_blocked(self):
        deb = self.build('replacement', f'Conflicts: {CORE}\nReplaces: {CORE}\n')
        for action in ('-i', '--install', '--unpack'):
            with self.subTest(action=action):
                result = self.run_hook([action, deb])
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('dpkg-package-guard: refusing', result.stderr)
                self.assertIn('Conflicts: ' + CORE, result.stderr)
                self.assert_core()
                self.assertFalse((self.root / 'usr/share/replacement/marker').exists())

    def test_recursive_and_multiple(self):
        archives = self.directory / 'archives with spaces'
        archives.mkdir()
        deb = self.build('replacement', f'Conflicts: {CORE}\n')
        deb.rename(archives / 'replacement.deb')
        result = self.run_hook(['-iR', archives])
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('dpkg-package-guard: refusing', result.stderr)
        safe = self.build('ordinary')
        result = self.run_hook(['-i', '--', safe, archives / 'replacement.deb'])
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.root / 'usr/share/ordinary/marker').exists())
        self.assert_core()

    def test_normal_and_upgrade(self):
        for deb in (self.build('ordinary'), self.build('replace-only', f'Replaces: {CORE}\n'),
                    self.build(CORE), self.build(CORE, version='2.0')):
            result = self.run_hook(['-i', deb])
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assert_core()

    def test_relation_matching(self):
        for relation, blocked in ((CORE, True), (CORE + ' (>= 1.0)', True),
                                  (CORE + ' (>> 1.0)', False), (CORE + ' (<< 2.0)', True),
                                  (CORE + ' (<= 0.9)', False), (CORE + ' (= 1.0)', True),
                                  (CORE + ':any', True), (CORE + ':arm64', False), ('unrelated', False)):
            with self.subTest(relation=relation):
                deb = self.build('replacement', 'Conflicts: ' + relation + '\n')
                argv = ['dpkg', '--admindir=' + str(self.root / 'var/lib/dpkg'), '-i', str(deb)]
                self.assertEqual(bool(guard.installation_conflicts(argv, {CORE})), blocked)

    def test_empty_disabled_and_invalid_policy(self):
        for config, packages, warning in (
                ('[guard]\nenabled = true\n', '# no protected packages\n', False),
                ('[guard]\nenabled = false\n', 'invalid:name\n', False),
                ('[guard]\nenabled = invalid\n', CORE + '\n', True)):
            with self.subTest(config=config, packages=packages):
                self.config.write_text(config)
                self.packages.write_text(packages)
                # Reinstall first; the following removal must be allowed by the hook.
                self.install(self.build(CORE))
                result = self.run_hook(['--remove', CORE])
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual('protection skipped' in result.stderr, warning)
                self.assertFalse((self.root / 'usr/share' / CORE / 'marker').exists())

    def test_policy_reloaded(self):
        result = self.run_hook(['--remove', CORE])
        self.assertIn('refusing operation', result.stderr)
        self.assert_core()
        self.packages.write_text('other-package\n')
        result = self.run_hook(['--remove', CORE])
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertFalse((self.root / 'usr/share' / CORE / 'marker').exists())

    def test_missing_archive_and_removal(self):
        result = self.run_hook(['-i', self.directory / 'missing.deb'])
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('dpkg-package-guard: refusing', result.stderr)
        for action in ('--remove', '--purge'):
            result = self.run_hook([action, CORE])
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('dpkg-package-guard: refusing', result.stderr)
            self.assert_core()


if __name__ == '__main__':
    unittest.main()
