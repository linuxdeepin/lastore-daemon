package check

import (
	"os"
	"text/template"
)

type ShellTemplate struct {
	Verbose bool
}

type EmulationMainShell struct {
	ShellTemplate
	RunCommand     string
	MainCommand    string
	PointRootPath  string
	SourcePoint    []string
	DebInstallPath string
}

/*!
 * @brief RenderMainShell
 * @param output output file
 * @param save save file
 * @return error
 */
func (ts *EmulationMainShell) RenderMainShell(save string) error {
	tpl, err := template.New("emumainshell").Parse(emulationMainShell_TMPL)
	if err != nil {
		logger.Fatalf("parse deb shell template failed! %+v %+v", err, tpl)
		return err
	}

	logger.Debugf("create save file: %s", save)
	savePath, err := os.Create(save)
	if err != nil {
		logger.Fatalf("save to %s failed!", save)
		return err
	}

	defer savePath.Close()
	logger.Debugf("object: %+v", ts)

	if err := tpl.Execute(savePath, ts); err != nil {
		logger.Debugf("tmp failed: %+v", err)
		return err
	}

	if err := savePath.Chmod(0755); err != nil {
		logger.Debugf("chmod failed: %+v", err)
		return err
	}

	return nil
}

// RenderDebInstallShell
/*!
 * @brief RenderDebInstallShell
 * @param output output file
 * @param save save file
 * @return error
 */
func (ts *EmulationMainShell) RenderDebInstallShell(save string) error {
	tpl, err := template.New("emudebinstallshell").Parse(emulationDebInstallShell_TMPL)
	if err != nil {
		logger.Fatalf("parse deb shell template failed! %+v %+v", err, tpl)
		return err
	}

	logger.Debugf("create save file: %s", save)
	savePath, err := os.Create(save)
	if err != nil {
		logger.Fatalf("save to %s failed!", save)
		return err
	}

	defer savePath.Close()
	logger.Debugf("object: %+v", ts)

	if err := tpl.Execute(savePath, ts); err != nil {
		logger.Debugf("tmp failed: %+v", err)
		return err
	}

	if err := savePath.Chmod(0755); err != nil {
		logger.Debugf("chmod failed: %+v", err)
		return err
	}

	return nil
}

const emulationMainShell_TMPL = `#!/bin/bash
{{if .Verbose }}set -x {{end}}
{{range $idx, $element := .SourcePoint}}
{{- if len $element }}
mount -t overlay overlay  -o "lowerdir=/{{$element}},upperdir={{$.PointRootPath}}/upperdir/{{$element}},workdir={{$.PointRootPath}}/workdir/{{$element}}" "{{$.PointRootPath}}/rootfs/{{$element}}"
if [ "$?" -ne 0 ]
then
	merger_dir=$(mktemp -d)
	mergerfs {{ $element }} $merger_dir
	mount -t overlay overlay  -o "lowerdir=/$merger_dir,upperdir={{$.PointRootPath}}/upperdir/$merger_dir,workdir={{$.PointRootPath}}/workdir/$merger_dir" "{{$.PointRootPath}}/rootfs/$merger_dir"
fi
{{- end}}
{{end}}
touch "{{.PointRootPath}}/rootfs/dev/tty"
mount -o bind /dev/tty "{{.PointRootPath}}/rootfs/dev/tty"

touch "{{.PointRootPath}}/rootfs/dev/null"
mount -o bind /dev/null "{{.PointRootPath}}/rootfs/dev/null"

touch "{{.PointRootPath}}/rootfs/dev/zero"
mount -o bind /dev/zero "{{.PointRootPath}}/rootfs/dev/zero"

touch "{{.PointRootPath}}/rootfs/dev/full"
mount -o bind /dev/full "{{.PointRootPath}}/rootfs/dev/full"

touch "{{.PointRootPath}}/rootfs/dev/random"
mount -o bind /dev/random "{{.PointRootPath}}/rootfs/dev/random"

touch "{{.PointRootPath}}/rootfs/dev/urandom"
mount -o bind /dev/urandom "{{.PointRootPath}}/rootfs/dev/urandom"

if ! [ -f "{{.PointRootPath}}/rootfs/{{.RunCommand}}" ]
then
    cp -fv {{.RunCommand}} "{{.PointRootPath}}/rootfs/{{.RunCommand}}"
fi

chroot "{{.PointRootPath}}/rootfs" /bin/bash "{{.RunCommand}}"

exitcode="$?"

sync && sync

umount "{{.PointRootPath}}/rootfs/dev/tty"
rm -f "{{.PointRootPath}}/rootfs/dev/tty"

umount "{{.PointRootPath}}/rootfs/dev/null"
rm -f "{{.PointRootPath}}/rootfs/dev/null"

umount "{{.PointRootPath}}/rootfs/dev/zero"
rm -f "{{.PointRootPath}}/rootfs/dev/zero"

umount "{{.PointRootPath}}/rootfs/dev/full"
rm -f "{{.PointRootPath}}/rootfs/dev/full"

umount "{{.PointRootPath}}/rootfs/dev/random"
rm -f "{{.PointRootPath}}/rootfs/dev/random"

umount "{{.PointRootPath}}/rootfs/dev/urandom"
rm -f "{{.PointRootPath}}/rootfs/dev/urandom"

exit $exitcode
`

const emulationDebInstallShell_TMPL = `#!/bin/bash
{{if .Verbose }}set -x {{end}}
mount -t proc proc /proc && dpkg -i {{.DebInstallPath}}/*.deb
`

const emulationDebListShell_TMPL = `#!/bin/bash
{{if .Verbose }}set -x {{end}}
mount -t proc proc /proc && dpkg -l 
`
