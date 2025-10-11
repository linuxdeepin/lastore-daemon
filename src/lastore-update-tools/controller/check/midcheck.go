package check

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

// CheckPkgDependency uses apt-get check to verify system package dependencies
func CheckPkgDependency() error {
	// Use apt-get check with NoLocking option to avoid lock conflicts
	cmd := exec.Command("apt-get", "check", "-o", "Debug::NoLocking=1")
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		// Parse the error using the existing apt error parser
		parseErr := apt.ParsePkgSystemError(outBuf.Bytes(), errBuf.Bytes())
		return parseErr
	}

	return nil
}

func CheckCoreFileExist(coreFilePath string) error {
	if err := fs.CheckFileExistState(coreFilePath); err != nil {
		return err
	}
	coreFile, err := ioutil.ReadFile(coreFilePath)
	if err != nil {
		return fmt.Errorf("unable to read coreFilePath:%s file: %v", coreFilePath, err)
	}
	coreFileList := strings.Split(string(coreFile), "\n")
	for _, path := range coreFileList {
		path = strings.TrimSpace(path)
		if strings.HasPrefix(path, "#") {
			continue
		}
		if err := fs.CheckFileExistState(path); err != nil {
			logger.Errorf("core file %s not exist:%v", path, err)
			return fmt.Errorf("core file %s not exist:%v", path, err)
		}
	}
	return nil
}

func CheckDebListInstallState(midpkgs map[string]*cache.AppTinyInfo, pkginfo *cache.AppInfo, checkStage string, listType string) error {

	if _, pkgexist := midpkgs[pkginfo.Name]; !pkgexist {
		logger.Warningf("%s (%s) is missing in system.", listType, pkginfo.Name)
		return &system.JobError{
			ErrType:      system.ErrorCheckPkgNotFound,
			ErrDetail:    fmt.Sprintf("%s (%s) is missing in system", listType, pkginfo.Name),
			IsCheckError: true,
		}
	}

	sysPkginfo := midpkgs[pkginfo.Name]
	if pkginfo.Need != "skipstate" && pkginfo.Need != "exist" {
		if !sysPkginfo.State.CheckOK() {
			logger.Warningf("%s (%s) state is err: %v.", listType, pkginfo.Name, sysPkginfo.State)
			return &system.JobError{
				ErrType:      system.ErrorCheckPkgState,
				ErrDetail:    fmt.Sprintf("%s (%s) state is err: %v", listType, pkginfo.Name, sysPkginfo.State),
				IsCheckError: true,
			}
		}
	}

	if checkStage != "precheck" {
		if pkginfo.Need != "skipversion" && pkginfo.Need != "exist" {
			if sysPkginfo.Version != pkginfo.Version {
				logger.Warningf("%s (%s) version is err: %s.", listType, pkginfo.Name, sysPkginfo.Version)
				return &system.JobError{
					ErrType:      system.ErrorCheckPkgVersion,
					ErrDetail:    fmt.Sprintf("%s (%s) version is err: %s", listType, pkginfo.Name, sysPkginfo.Version),
					IsCheckError: true,
				}
			}
		}
	}

	return nil
}
