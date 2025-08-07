package update

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/sysinfo"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
)

var (
	sRealArch string
)

func init() {
	cmd := exec.Command("/usr/bin/dpkg", "--print-architecture")

	var output bytes.Buffer
	cmd.Stdout = &output
	err := cmd.Run()
	if err == nil {
		sRealArch = strings.TrimSpace(output.String())
	}
}

func UpdatePkgInstall(DebPath string) error {

	// install package
	// limit install time 1h
	_, err := runcmd.RunnerOutput(3600, "bash", "-c", fmt.Sprintf("dpkg --debug=777777  --force-downgrade --force-hold --force-confask --force-confdef --force-all -i %s/*.deb", DebPath+"/deb"))
	// outputStream, err := runcmd.RunnerOutput(3600, "bash", "-c", fmt.Sprintf("dpkg --debug=777777  --force-downgrade --force-hold --force-bad-version --force-confask --force-confdef --force-all -i %s/*.deb", CacheCfg.WorkStation+"/deb"))
	if err != nil {
		log.Errorln(err)
		return err
	}
	return nil
}

func getNewFileName(version, old string) (string, error) {

	strRight := strings.LastIndex(old, "_")
	if strRight < 0 {
		return "", fmt.Errorf("format error")
	}
	strLeft := strings.LastIndex(old[:strRight], "_")
	if strLeft < 0 {
		return "", fmt.Errorf("format error")
	}

	newVersion := old[:strLeft+1] + version + old[strRight:]
	return newVersion, nil
}

func getRealFileName(name, version, arch string) string {
	if arch == "" {
		arch = "all"
	}
	return fmt.Sprintf("%s_%s_%s.deb", name, version, arch)
}

func tryTwoFileName(DebPath, Name, Version, Arch string) (string, error) {
	vFilePath := DebPath + "/" + getRealFileName(Name, Version, Arch)
	log.Warnf("try 1 search:%s", vFilePath)
	if err := fs.CheckFileExistState(vFilePath); err != nil {
		// udeb ?
		vFilePath = fmt.Sprintf("%s/%s_%s_*.deb", DebPath, Name, Version)
		log.Warnf("try 2 search:%s", vFilePath)
		if rs, err := filepath.Glob(vFilePath); err != nil || len(rs) <= 0 {
			return "", fmt.Errorf("pkg %v is not found install package", Name)
		} else {
			log.Warnf("get filepath:%v", rs)
			return vFilePath, nil
		}
	} else {
		return vFilePath, nil
	}
}

func getNewVersion(idx int, sep, version string) string {
	return version[:idx] + sep + version[idx+1:]
}

func UpdatePackageInstall(metainfo *cache.CacheInfo) error {

	// install package
	// limit install time 1h
	pkgInstList := ""
	for _, pkginfo := range metainfo.UpdateMetaInfo.PkgList {
		if pkginfo.FilePath != "" {
			vArch := pkginfo.Arch
			if pkginfo.Arch == "sw_64" || sRealArch == "sw_64" {
				vArch = "sw%5f64"
			}
			epochIdx := strings.Index(pkginfo.Version, ":")
			if epochIdx < 0 {
				if err := fs.CheckFileExistState(pkginfo.FilePath); err != nil {
					if strPath, err := tryTwoFileName(metainfo.UpdateMetaInfo.PkgDebPath, pkginfo.Name, pkginfo.Version, vArch); err != nil {
						return err
					} else {
						pkgInstList = pkgInstList + " " + strPath
					}

				} else {
					pkgInstList = pkgInstList + " " + pkginfo.FilePath
				}
			} else {
				if err := fs.CheckFileExistState(pkginfo.FilePath); err != nil {
					newVersion := getNewVersion(epochIdx, "%3a", pkginfo.Version)
					newPath, err := getNewFileName(newVersion, pkginfo.FilePath)
					if err != nil {
						return fmt.Errorf("pkg %v is not found install package", pkginfo.Name)
					}
					if err := fs.CheckFileExistState(newPath); err != nil {
						if strPath, err := tryTwoFileName(metainfo.UpdateMetaInfo.PkgDebPath, pkginfo.Name, newVersion, vArch); err != nil {
							return err
						} else {
							pkgInstList = pkgInstList + " " + strPath
						}
						// vFilePath := metainfo.UpdateMetaInfo.PkgDebPath + "/" + getRealFileName(pkginfo.Name, newVersion, vArch)
						// log.Warnf("search package:%s", vFilePath)
						// if err := fs.CheckFileExistState(vFilePath); err != nil {
						// 	return fmt.Errorf("pkg %v is not found install package", pkginfo.Name)
						// } else {
						// 	pkgInstList = pkgInstList + " " + vFilePath
						// }

					} else {
						pkgInstList = pkgInstList + " " + newPath
					}

				} else {
					pkgInstList = pkgInstList + " " + pkginfo.FilePath
				}
			}

		} else {
			return fmt.Errorf("pkg %v is not found install package", pkginfo.Name)
		}

	}
	Argv := fmt.Sprintf("/usr/bin/apt-get -y -o Acquire::Retries=3 -c /var/lib/lastore/apt_v2_common.conf --allow-downgrades --allow-change-held-packages -o Dpkg::Options::=\"--debug=770007\" -o Dir::Etc::SourceList=/dev/null -o Dir::Etc::SourceParts=/dev/null install %s >%s 2>&1", pkgInstList, metainfo.WorkStation+"/dpkg.log")
	_, err := runcmd.RunnerOutputEnv(3600, "/usr/bin/bash", []string{"DEBIAN_FRONTEND=noninteractive", "DEBCONF_NONINTERACTIVE_SEEN=true", "DEBIAN_PRIORITY=critical"}, "-c", Argv)
	// _, err := runcmd.RunnerOutput(3600, "apt-get", fmt.Sprintf("env DEBIAN_FRONTEND=noninteractive DEBCONF_NONINTERACTIVE_SEEN=true DEBIAN_PRIORITY=critical apt-get install  --force-downgrade --force-hold  --force-confdef --force-confold -i %s 2> %s", pkgInstList, metainfo.WorkStation+"/dpkg.log"))
	if err != nil {
		log.Errorln(err)
		return err
	}
	return nil
}

func UpdatePackagePurge(metainfo *cache.CacheInfo) error {

	sysPkgList := map[string]*cache.AppTinyInfo{}
	if err := sysinfo.GetCurrInstPkgStat(sysPkgList); err != nil {
		return err
	}
	pkgPurgeList := ""
	for _, pkginfo := range metainfo.UpdateMetaInfo.PurgeList {

		if _, ok := sysPkgList[pkginfo.Name]; !ok {
			if metainfo.InternalState.IsPurgeState.IsFirstRun() {
				return fmt.Errorf("not found purge package %s", pkginfo.Name)
			} else {
				log.Warnf("purge package skip:%s", pkginfo.Name)
				continue
			}

		} else {
			switch pkginfo.Need {
			case "exist", "skipversion":
				pkgPurgeList = pkgPurgeList + " " + pkginfo.Name
			case "skipstate", "strict", "":
				pkgPurgeList = pkgPurgeList + " " + pkginfo.Name
			default:
				continue
			}
		}

	}
	log.Debugf("purge list:%s", pkgPurgeList)
	if pkgPurgeList == "" {
		if metainfo.InternalState.IsPurgeState == cache.P_OK {
			return nil
		} else {
			return fmt.Errorf("not found purge list")
		}

	}
	_, err := runcmd.RunnerOutputEnv(3600, "/usr/bin/dpkg", []string{"DEBIAN_FRONTEND=noninteractive", "DEBCONF_NONINTERACTIVE_SEEN=true", "DEBIAN_PRIORITY=critical"}, fmt.Sprintf("--debug=770007  --force-downgrade --force-hold --force-all --remove %s >%s 2>&1", pkgPurgeList, metainfo.WorkStation+"/purge.log"))
	if err != nil {
		log.Errorln(err)
		return err
	}
	return nil
}
