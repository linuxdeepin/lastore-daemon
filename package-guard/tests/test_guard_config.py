"""Shared policy contract and APT protocol tests; never access system policy."""
from contextlib import redirect_stderr
import importlib.util
import importlib.machinery
import io
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


HERE = Path(__file__).resolve().parent
HOOK_DIR = HERE.parents[1] / 'usr' / 'libexec' / 'lastore-daemon'


def load_hook(name):
    filename = name
    path = HOOK_DIR / filename
    loader = importlib.machinery.SourceFileLoader(name, str(path))
    spec = importlib.util.spec_from_loader(name, loader)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


apt = load_hook('apt-package-guard')
dpkg = load_hook('dpkg-package-guard')


class PolicyFiles(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix='guard-policy-')
        self.addCleanup(temporary.cleanup)
        self.directory = Path(temporary.name)
        self.config = self.directory / 'guard.conf'
        self.packages = self.directory / 'protected-packages.list'
        self.config.write_text('[guard]\nenabled = true\n', encoding='utf-8')
        self.packages.write_text('dde-shell\n', encoding='utf-8')

    def policy(self, hook, expected, warning=False):
        error = io.StringIO()
        with redirect_stderr(error):
            actual = hook.load_protected_packages(self.config, self.packages)
        self.assertEqual(actual, expected)
        if warning:
            self.assertIn(hook.__name__ + ': warning:', error.getvalue())
            self.assertIn(str(self.directory), error.getvalue())
        else:
            self.assertEqual(error.getvalue(), '')


class PolicyTest(PolicyFiles):
    def test_valid_lists_and_reload(self):
        for content, expected in (
                ('  # comment\n\n dde-shell \ndde-shell\nlightdm\n', {'dde-shell', 'lightdm'}),
                ('', set()), (' \n\t\n', set()), ('# comment\n  # another\n', set()),
                ('package+suffix\npackage.name\npackage-01\n',
                 {'package+suffix', 'package.name', 'package-01'})):
            self.packages.write_text(content, encoding='utf-8')
            for hook in (apt, dpkg):
                with self.subTest(hook=hook.__name__, content=content):
                    self.policy(hook, expected)

    def test_disabled_never_opens_list(self):
        self.config.write_text('# comment\n; comment\n[guard]\nenabled = false\n')
        self.packages.unlink()
        for hook in (apt, dpkg):
            with patch('builtins.open', wraps=open) as opened:
                self.policy(hook, set())
            self.assertEqual(opened.call_count, 1)
            self.assertEqual(opened.call_args.args[0], self.config)

    def test_invalid_configurations(self):
        cases = (
            '', '[guard]\n', '[guard]\nenabled = yes\n',
            '[guard]\nenabled = TRUE\n', '[guard]\nenabled = %(value)s\n',
            'not an ini file', '[guard]\nEnabled = true\n',
        )
        for content in cases:
            self.config.write_text(content, encoding='utf-8')
            for hook in (apt, dpkg):
                with self.subTest(hook=hook.__name__, content=content):
                    self.policy(hook, set(), warning=True)

    def test_invalid_list_lines_are_skipped(self):
        for bad in ('dde-shell:amd64', 'dde-*', 'Dde-shell', 'dde-shell (>= 1)',
                    'dde-shell # comment', '; comment', 'two packages', '-name', 'a'):
            self.packages.write_text('lightdm\n' + bad + '\n', encoding='utf-8')
            for hook in (apt, dpkg):
                with self.subTest(hook=hook.__name__, bad=bad):
                    self.policy(hook, {'lightdm'}, warning=True)

    def test_extra_sections_and_options_are_ignored(self):
        for content in (
                '[guard]\nenabled = true\nextra = value\n[other]\nanything = yes\n',
                '[guard]\nenabled = false\nextra = value\n[other]\nanything = yes\n'):
            self.config.write_text(content, encoding='utf-8')
            for hook in (apt, dpkg):
                self.policy(hook, {'dde-shell'} if 'true' in content else set())

    def test_duplicate_settings_use_final_value(self):
        for content, expected in (
                ('[guard]\nenabled = true\nenabled = false\n', set()),
                ('[guard]\nenabled = false\nenabled = true\n', {'dde-shell'}),
                ('[guard]\nenabled = true\n[guard]\nenabled = false\n', set()),
                ('[guard]\nenabled = false\n[guard]\nenabled = true\n', {'dde-shell'}),
                ('[guard]\nenabled = true\n[guard]\nextra = value\n', {'dde-shell'})):
            self.config.write_text(content, encoding='utf-8')
            for hook in (apt, dpkg):
                self.policy(hook, expected)

    def test_missing_unreadable_and_undecodable_files(self):
        for target in (self.config, self.packages):
            original = target.read_bytes()
            for hook in (apt, dpkg):
                with self.subTest(hook=hook.__name__, target=target):
                    target.unlink()
                    self.policy(hook, set(), warning=True)
                    target.write_bytes(b'\xff')
                    self.policy(hook, set(), warning=True)
                    target.write_bytes(original)
                    real_open = open

                    def denied(path, *args, **kwargs):
                        if path == target:
                            raise PermissionError('test permission denied')
                        return real_open(path, *args, **kwargs)

                    with patch('builtins.open', side_effect=denied):
                        self.policy(hook, set(), warning=True)

    def test_dpkg_skips_process_and_database_work(self):
        for config, packages, warning in (
                ('[guard]\nenabled = false\n', 'invalid:name\n', False),
                ('[guard]\nenabled = true\n', '', False),
                ('[guard]\nenabled = invalid\n', 'dde-shell\n', True)):
            self.config.write_text(config)
            self.packages.write_text(packages)
            for action in ('remove', 'purge', 'install', 'unpack'):
                with self.subTest(config=config, action=action), \
                        patch.dict(dpkg.os.environ, {'DPKG_HOOK_ACTION': action}), \
                        patch.object(dpkg, 'find_dpkg_argv') as arguments, \
                        patch.object(dpkg, 'checked_output') as database, \
                        redirect_stderr(io.StringIO()) as error:
                    self.assertEqual(dpkg.main(self.config, self.packages), 0)
                    arguments.assert_not_called()
                    database.assert_not_called()
                    self.assertEqual(bool(error.getvalue()), warning)

    def test_dpkg_unrelated_action_does_not_read_config(self):
        with patch.dict(dpkg.os.environ, {'DPKG_HOOK_ACTION': 'configure'}), \
                patch.object(dpkg, 'load_protected_packages') as loader:
            self.assertEqual(dpkg.main(self.config, self.packages), 0)
            loader.assert_not_called()


class AptTest(PolicyFiles):
    def run_apt(self, text):
        stream = io.StringIO(text)
        error = io.StringIO()
        with redirect_stderr(error):
            code = apt.main(self.config, self.packages, stream)
        return code, error.getvalue(), stream

    def test_protocol(self):
        for body, code in (
                ('dde-shell 1 > - **REMOVE**\n', 1),
                ('dde-shell:amd64 1 > - **REMOVE**\n', 1),
                ('other-package 1 > - **REMOVE**\n', 0),
                ('dde-shell 1 = 1 **CONFIGURE**\n', 0),
                ('dde-shell 1 = 1 /tmp/package.deb\n', 0),
                ('dde-shell 1 > - **REMOVE**\nother-package 1 > - **REMOVE**\n', 1),
                ('malformed record\n', 1)):
            with self.subTest(body=body):
                actual, error, _ = self.run_apt('VERSION 2\nAPT::Architecture=amd64\n\n' + body)
                self.assertEqual(actual, code)
                if '**REMOVE**' in body and body.startswith('dde-shell'):
                    self.assertIn('refusing removal of: dde-shell', error)
        for text in ('VERSION 1\n', 'VERSION 2\nmissing terminator\n'):
            self.assertEqual(self.run_apt(text)[0], 1)

    def test_skipped_protection_drains_malformed_input(self):
        for config, packages, warning in (
                ('[guard]\nenabled = false\n', 'invalid:name\n', False),
                ('[guard]\nenabled = true\n', '# empty\n', False),
                ('bad ini', 'dde-shell\n', True),
                ('[guard]\nenabled = true\n', 'invalid:name\n', True)):
            self.config.write_text(config)
            self.packages.write_text(packages)
            content = 'VERSION 1\n' + 'x' * 200000
            code, error, stream = self.run_apt(content)
            self.assertEqual(code, 0)
            self.assertEqual(stream.tell(), len(content))
            self.assertEqual(bool(error), warning)

    def test_main_reloads_policy(self):
        text = 'VERSION 2\n\ndde-shell 1 > - **REMOVE**\n'
        self.assertEqual(self.run_apt(text)[0], 1)
        self.packages.write_text('other-package\n')
        self.assertEqual(self.run_apt(text)[0], 0)
        self.packages.write_text('dde-shell\n')
        self.assertEqual(self.run_apt(text)[0], 1)


if __name__ == '__main__':
    unittest.main()
