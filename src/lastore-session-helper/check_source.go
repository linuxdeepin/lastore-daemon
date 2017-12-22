package main

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/gettext"
	"pkg.deepin.io/lib/xdg/basedir"

	log "github.com/cihub/seelog"
)

const (
	aptSource       = "/etc/apt/sources.list"
	aptSourceOrigin = aptSource + ".origin"
	aptSourceDir    = aptSource + ".d"
)

var disableSourceCheckFile = filepath.Join(basedir.GetUserConfigDir(), "deepin",
	"lastore-session-helper", "disable-source-check")

func (l *Lastore) checkSource() {
	const sourceCheckedFile = "/tmp/lastore-session-helper-source-checked"

	if _, err := os.Stat(sourceCheckedFile); os.IsNotExist(err) {
		ok := doCheckSource()
		log.Info("checkSource:", ok)
		if !ok {
			notifySourceModified(l.createSourceModifiedActions())
		}

		err = touchFile(sourceCheckedFile)
		if err != nil {
			log.Warn("failed to touch source-checked file:", err)
		}
	}
}

func (l *Lastore) createSourceModifiedActions() []Action {
	return []Action{
		{
			Id:   "restore",
			Name: gettext.Tr("Restore"),
			Callback: func() {
				log.Info("restore source")
				err := l.updater.RestoreSystemSource()
				if err != nil {
					log.Warnf("failed to restore source:", err)
				}
			},
		},
		{
			Id:   "Cancel",
			Name: gettext.Tr("Cancel"),
			Callback: func() {
				log.Info("cancel restore source")
			},
		},
	}
}

const (
	propNameSourceCheckEnabled = "SourceCheckEnabled"
)

func (l *Lastore) SetSourceCheckEnabled(val bool) error {
	if l.SourceCheckEnabled == val {
		return nil
	}

	if val {
		// enable
		err := os.Remove(disableSourceCheckFile)
		if err != nil {
			return err
		}
	} else {
		// disable
		err := touchFile(disableSourceCheckFile)
		if err != nil {
			return err
		}
	}

	l.SourceCheckEnabled = val
	dbus.NotifyChange(l, propNameSourceCheckEnabled)
	return nil
}

// return is source ok?
func doCheckSource() bool {
	originLines, err := loadAptSource(aptSourceOrigin)
	if err != nil {
		// no origin
		return true
	}

	lines, err := loadAptSource(aptSource)
	if err != nil {
		log.Warnf("failed to load apt source:", err)
		return false
	}

	if !linesEqual(originLines, lines) {
		return false
	}

	fileInfoList, err := ioutil.ReadDir(aptSourceDir)
	if err != nil {
		log.Warnf("read apt source dir err: %v", err)
	}
	for _, fileInfo := range fileInfoList {
		if fileInfo.IsDir() {
			continue
		}

		ext := filepath.Ext(fileInfo.Name())
		if ext == ".list" || ext == ".sources" {
			return false
		}
	}

	return true
}

func linesEqual(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

func loadAptSource(filename string) ([][]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	var lines [][]byte
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if bytes.HasPrefix(line, []byte{'#'}) || len(line) == 0 {
			// ignore comment and empty line
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
