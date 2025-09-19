package check

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

const corePkgListConFilePath = CheckBaseDir + "core-pkg-list.conf"

func LoadCorePkgList() []string {
	var pkgs []string
	if _, err := os.Stat(corePkgListConFilePath); os.IsNotExist(err) {
		logger.Warningf("core pkg list config file %s not found", corePkgListConFilePath)
		return pkgs
	}

	content, err := ioutil.ReadFile(corePkgListConFilePath)
	if err != nil {
		logger.Warningf("failed to read core pkg list config file :%v", err)
		return pkgs
	}

	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			pkgs = append(pkgs, line)
		}
	}
	return pkgs
}

// DONE:(heysion) maybe modify check all and return once
func CheckPkgDependency(sysCurrPackage map[string]*cache.AppTinyInfo) (int64, error) {
	breakDepends := false
	var breakDependsError error
	corePkgList := LoadCorePkgList()
	for _, pkgName := range corePkgList {
		pkginfo, err := sysCurrPackage[pkgName]
		if !err || pkginfo == nil || !pkginfo.State.CheckOK() {
			breakDepends = true
			logger.Debugf("dependency error: %v", pkgName)
			if breakDependsError != nil {
				breakDependsError = fmt.Errorf("%v dependency error: %v:%v", breakDependsError, pkgName, pkginfo)
			} else {
				breakDependsError = fmt.Errorf("dependency error: %v:%v", pkgName, pkginfo)
			}
		}
	}
	if breakDepends {
		return ecode.CHK_PKG_DEPEND_ERROR, fmt.Errorf("found package state err: %v", breakDependsError)
	}
	return ecode.CHK_PROGRAM_SUCCESS, nil
}

//TODO:(DingHao) 可后期重构,优化传参
// FIXME:(Dinghao) 使用 CheckCorelistInstallState
// func CheckPkglistInstallState(midpkgs map[string]*cache.AppTinyInfo, pkginfo *cache.AppInfo) (int, error) {
// 	if _, pkgexist := midpkgs[pkginfo.Name]; !pkgexist {
// 		logger.Errorf("pkglist (%s) is missing in system.", pkginfo.Name)
// 		return ecode.CHK_PKGLIST_INEXISTENCE, fmt.Errorf("pkglist (%s) is missing in system", pkginfo.Name)
// 	}

// 	sysPkginfo := midpkgs[pkginfo.Name]

// 	if pkginfo.Need != "skipstate" && pkginfo.Need != "exist" {
// 		if sysPkginfo.State != cache.Installed && sysPkginfo.State != cache.InstallHolded {
// 			logger.Errorf("pkglist (%s) state is err: %d.", pkginfo.Name, sysPkginfo.State)
// 			return ecode.CHK_PKGLIST_ERR_STATE, fmt.Errorf("pkglist (%s) state is err: %d", pkginfo.Name, sysPkginfo.State)
// 		}
// 	}

// 	if pkginfo.Need != "skipversion" && pkginfo.Need != "exist" {
// 		if sysPkginfo.Version != pkginfo.Version {
// 			logger.Errorf("pkglist (%s) version is err: %s.", pkginfo.Name, sysPkginfo.Version)
// 			return ecode.CHK_PKGLIST_ERR_VERSION, fmt.Errorf("pkglist (%s) version is err: %s", pkginfo.Name, sysPkginfo.Version)
// 		}
// 	}

// 	return ecode.CHK_PROGRAM_SUCCESS, nil
// }

func CheckCoreFileExist(coreFilePath string) (int64, error) {
	if err := fs.CheckFileExistState(coreFilePath); err != nil {
		return ecode.CHK_PROGRAM_ERROR, err
	}
	coreFile, err := ioutil.ReadFile(coreFilePath)
	if err != nil {
		return ecode.CHK_PROGRAM_ERROR, fmt.Errorf("unable to read coreFilePath:%s file: %v", coreFilePath, err)
	}
	coreFileList := strings.Split(string(coreFile), "\n")
	for _, path := range coreFileList {
		path = strings.TrimSpace(path)
		if strings.HasPrefix(path, "#") {
			continue
		}
		if err := fs.CheckFileExistState(path); err != nil {
			logger.Errorf("core file %s not exist:%v", path, err)
			return ecode.CHK_CORE_FILE_MISS, fmt.Errorf("core file %s not exist:%v", path, err)
		}
	}
	return ecode.CHK_PROGRAM_SUCCESS, nil
}

// func CheckCorelistInstallState(midpkgs map[string]*cache.AppTinyInfo, pkginfo *cache.AppInfo) (int, error) {
// 	if _, pkgexist := midpkgs[pkginfo.Name]; !pkgexist {
// 		logger.Warningf("corelist (%s) is missing in system.", pkginfo.Name)
// 		return ecode.CHK_CORE_PKG_NOTFOUND, fmt.Errorf("corelist (%s) is missing in system", pkginfo.Name)
// 	}

// 	sysPkgInfo := midpkgs[pkginfo.Name]

// 	if pkginfo.Need != "skipstate" && pkginfo.Need != "exist" {
// 		if sysPkgInfo.State != cache.Installed && sysPkgInfo.State != cache.InstallHolded {
// 			logger.Warningf("corelist (%s) state is err: %d.", pkginfo.Name, sysPkgInfo.State)
// 			return ecode.CHK_CORE_PKG_ERR_STATE, fmt.Errorf("corelist (%s) state is err: %d", pkginfo.Name, sysPkgInfo.State)
// 		}
// 	}

// 	if pkginfo.Need != "skipversion" && pkginfo.Need != "exist" {
// 		if sysPkgInfo.Version != pkginfo.Version {
// 			logger.Warningf("corelist (%s) version is err: %s.", pkginfo.Name, sysPkgInfo.Version)
// 			return ecode.CHK_CORE_PKG_ERR_VERSION, fmt.Errorf("corelist (%s) version is err: %s", pkginfo.Name, sysPkgInfo.Version)
// 		}
// 	}

// 	return ecode.CHK_PROGRAM_SUCCESS, nil
// }

// FIXME:(Dinghao) 使用 CheckCorelistInstallState
// func CheckOptionlistInstallState(midpkgs map[string]*cache.AppTinyInfo, pkginfo *cache.AppInfo) (int, error) {
// 	if _, pkgexist := midpkgs[pkginfo.Name]; !pkgexist {
// 		logger.Warningf("optionlist (%s) is missing in system.", pkginfo.Name)
// 		return ecode.CHK_OPTION_PKG_NOTFOUND, fmt.Errorf("optionlist (%s) is missing in system", pkginfo.Name)
// 	}
// 	sysPkginfo := midpkgs[pkginfo.Name]

// 	if pkginfo.Need != "skipstate" && pkginfo.Need != "exist" {
// 		if sysPkginfo.State != cache.Installed && sysPkginfo.State != cache.InstallHolded {
// 			logger.Warningf("optionlist (%s) state is err: %d.", pkginfo.Name, sysPkginfo.State)
// 			return ecode.CHK_OPTION_PKG_ERR_STATE, fmt.Errorf("optionlist (%s) state is err: %d", pkginfo.Name, sysPkginfo.State)
// 		}
// 	}

// 	if pkginfo.Need != "skipversion" && pkginfo.Need != "exist" {
// 		if sysPkginfo.Version != pkginfo.Version {
// 			logger.Warningf("optionlist (%s) version is err: %s.", pkginfo.Name, sysPkginfo.Version)
// 			return ecode.CHK_OPTION_PKG_ERR_VERSION, fmt.Errorf("optionlist (%s) version is err: %s", pkginfo.Name, sysPkginfo.Version)
// 		}
// 	}

// 	return ecode.CHK_PROGRAM_SUCCESS, nil
// }

func CheckDebListInstallState(midpkgs map[string]*cache.AppTinyInfo, pkginfo *cache.AppInfo, checkStage string, listType string) (int64, error) {

	var PKG_NOT_FOUND, PKG_ERR_STATE, PKG_ERR_VERSION int64
	switch listType {
	case "pkglist":
		PKG_NOT_FOUND = ecode.CHK_PKGLIST_INEXISTENCE
		PKG_ERR_STATE = ecode.CHK_PKGLIST_ERR_STATE
		PKG_ERR_VERSION = ecode.CHK_PKGLIST_ERR_VERSION
	case "corelist":
		PKG_NOT_FOUND = ecode.CHK_CORE_PKG_NOTFOUND
		PKG_ERR_STATE = ecode.CHK_CORE_PKG_ERR_STATE
		PKG_ERR_VERSION = ecode.CHK_CORE_PKG_ERR_VERSION
	case "optionlist":
		PKG_NOT_FOUND = ecode.CHK_OPTION_PKG_NOTFOUND
		PKG_ERR_STATE = ecode.CHK_OPTION_PKG_ERR_STATE
		PKG_ERR_VERSION = ecode.CHK_OPTION_PKG_ERR_VERSION
	}

	if _, pkgexist := midpkgs[pkginfo.Name]; !pkgexist {
		logger.Warningf("%s (%s) is missing in system.", listType, pkginfo.Name)
		return PKG_NOT_FOUND, fmt.Errorf("%s (%s) is missing in system", listType, pkginfo.Name)
	}

	sysPkginfo := midpkgs[pkginfo.Name]
	if pkginfo.Need != "skipstate" && pkginfo.Need != "exist" {
		if !sysPkginfo.State.CheckOK() {
			logger.Warningf("%s (%s) state is err: %v.", listType, pkginfo.Name, sysPkginfo.State)
			return PKG_ERR_STATE, fmt.Errorf("%s (%s) state is err: %v", listType, pkginfo.Name, sysPkginfo.State)
		}
	}

	if checkStage != "precheck" {
		if pkginfo.Need != "skipversion" && pkginfo.Need != "exist" {
			if sysPkginfo.Version != pkginfo.Version {
				logger.Warningf("%s (%s) version is err: %s.", listType, pkginfo.Name, sysPkginfo.Version)
				return PKG_ERR_VERSION, fmt.Errorf("%s (%s) version is err: %s", listType, pkginfo.Name, sysPkginfo.Version)
			}
		}
	}

	return ecode.CHK_PROGRAM_SUCCESS, nil
}
