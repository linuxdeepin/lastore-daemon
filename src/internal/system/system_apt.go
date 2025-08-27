// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

/*
This is system package manager need implement for porting
lastore-daemon
*/
package system

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"

	utils2 "github.com/linuxdeepin/go-lib/utils"
)

const (
	LastoreAptOrgConfPath      = "/var/lib/lastore/apt.conf"
	LastoreAptV2ConfPath       = "/var/lib/lastore/apt_v2.conf"
	LastoreAptV2CommonConfPath = "/var/lib/lastore/apt_v2_common.conf" // 该配置指定了通过lastore更新的deb包缓存路径
)

const (
	DutOnlineMetaConfPath = "/var/lib/lastore/online_meta.json" // 在线更新元数据
)

const (
	OnlineListPath = "/var/lib/apt/lists"
)

const (
	LocalCachePath = "/var/cache/lastore/archives"
)

// ListPackageFile list files path contained in the packages
func ListPackageFile(packages ...string) []string {
	desktopFiles, err := utils.FilterExecOutput(
		// #nosec G204
		exec.Command("dpkg", append([]string{"-L", "--"}, packages...)...),
		time.Second*2,
		func(string) bool { return true },
	)
	if err != nil {
		return nil
	}
	return desktopFiles
}

// QueryPackageDependencies return the directly dependencies
func QueryPackageDependencies(pkgId string) []string {
	// #nosec G204
	out, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f", "${Depends}", "--", pkgId).CombinedOutput()
	if err != nil {
		return nil
	}
	baseName := guestBasePackageName(pkgId)

	var r []string
	for _, line := range strings.Fields(string(out)) {
		if strings.Contains(line, baseName) {
			r = append(r, strings.Trim(line, ","))
		}
	}
	return r
}

/*
$ apt-config --format '%f=%v%n' dump  Dir
Dir=/
Dir::Cache=var/cache/apt
Dir::Cache::archives=archives/
Dir::Cache::srcpkgcache=srcpkgcache.bin
Dir::Cache::pkgcache=pkgcache.bin
*/
func GetArchivesDir(configPath string) (string, error) {
	binAptConfig, _ := exec.LookPath("apt-config")
	// #nosec G204
	output, err := exec.Command(binAptConfig, "-c", configPath, "--format", "%f=%v%n", "dump", "Dir").Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(output), "\n")
	tempMap := make(map[string]string)
	fieldsCount := 0
loop:
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			switch parts[0] {
			case "Dir", "Dir::Cache", "Dir::Cache::archives":
				tempMap[parts[0]] = parts[1]
				fieldsCount++
				if fieldsCount == 3 {
					break loop
				}
			}
		}
	}
	dir := tempMap["Dir"]
	if dir == "" {
		return "", errors.New("apt-config Dir is empty")
	}

	dirCache := tempMap["Dir::Cache"]
	if dirCache == "" {
		return "", errors.New("apt-config Dir::Cache is empty")
	}
	dirCacheArchives := tempMap["Dir::Cache::archives"]
	if dirCacheArchives == "" {
		return "", errors.New("apt-config Dir::Cache::Archives is empty")
	}

	return filepath.Join(dir, dirCache, dirCacheArchives), nil
}

// QueryFileCacheSize parsing the file total size(kb) of the path
func QueryFileCacheSize(path string) (float64, error) {
	// #nosec G204
	output, err := exec.Command("/usr/bin/du", "-s", path).Output()
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(output), "\t")
	if len(lines) != 0 {
		return strconv.ParseFloat(lines[0], 64)
	}
	return 0, nil
}

// QueryPackageDownloadSize parsing the total size of download archives when installing the packages.
// return arg0:需要下载的量;arg1:所有包的大小;arg2:error
func QueryPackageDownloadSize(updateType UpdateType, packages ...string) (float64, float64, error) {
	startTime := time.Now()
	if len(packages) == 0 {
		logger.Warningf("%v %v mode don't have can update package", updateType.JobType(), updateType)
		return SizeDownloaded, SizeDownloaded, NotFoundError("hasn't any packages")
	}
	downloadSize := new(float64)
	allPackageSize := new(float64)
	defer logger.Debugf("need download size:%v ,all package size:%v", *downloadSize, *allPackageSize)
	err := CustomSourceWrapper(updateType, func(path string, unref func()) error {
		defer func() {
			if unref != nil {
				unref()
			}
		}()
		var cmd *exec.Cmd
		if utils2.IsDir(path) {
			// #nosec G204
			cmd = exec.Command("/usr/bin/apt-get",
				append([]string{"-d", "-o", "Debug::NoLocking=1", "-c", LastoreAptV2CommonConfPath,
					"-o", fmt.Sprintf("%v=%v", "Dir::Etc::sourcelist", "/dev/null"),
					"-o", fmt.Sprintf("%v=%v", "Dir::Etc::SourceParts", path),
					"--print-uris", "--assume-no", "install", "--"}, packages...)...)
		} else {
			// #nosec G204
			cmd = exec.Command("/usr/bin/apt-get",
				append([]string{"-d", "-o", "Debug::NoLocking=1", "-c", LastoreAptV2CommonConfPath,
					"-o", fmt.Sprintf("%v=%v", "Dir::Etc::sourcelist", path),
					"-o", fmt.Sprintf("%v=%v", "Dir::Etc::SourceParts", "/dev/null"),
					"--print-uris", "--assume-no", "install", "--"}, packages...)...)
		}

		lines, err := utils.FilterExecOutput(cmd, time.Second*120, func(line string) bool {
			_, _, _err := parsePackageSize(line)
			return _err == nil
		})
		if err != nil && len(lines) == 0 {
			return fmt.Errorf("run:%v failed-->%v", cmd.Args, err)
		}

		if len(lines) != 0 {
			needDownloadSize, allSize, err := parsePackageSize(lines[0])
			if err != nil {
				logger.Warning(err)
				return err
			}
			*allPackageSize = allSize
			*downloadSize = needDownloadSize
		}
		return nil
	})
	if err != nil {
		logger.Warning(err)
		return SizeDownloaded, SizeDownloaded, err
	}
	logger.Debug("end QueryPackageDownloadSize duration:", time.Now().Sub(startTime))
	return *downloadSize, *allPackageSize, nil
}

// QuerySourceDownloadSize 根据更新类型(仓库),获取需要的下载量,return arg0:需要下载的量;arg1:所有包的大小;arg2:error
func QuerySourceDownloadSize(updateType UpdateType, pkgList []string) (float64, float64, error) {
	startTime := time.Now()
	downloadSize := new(float64)
	allPackageSize := new(float64)
	err := CustomSourceWrapper(updateType, func(path string, unref func()) error {
		defer func() {
			if unref != nil {
				unref()
			}
		}()
		var cmd *exec.Cmd
		if utils2.IsDir(path) {
			// #nosec G204
			cmd = exec.Command("/usr/bin/apt-get",
				append([]string{"dist-upgrade", "-d", "-o", "Debug::NoLocking=1", "-c", LastoreAptV2CommonConfPath, "--assume-no",
					"-o", fmt.Sprintf("%v=%v", "Dir::Etc::sourcelist", "/dev/null"),
					"-o", fmt.Sprintf("%v=%v", "Dir::Etc::SourceParts", path)}, pkgList...)...)
		} else {
			// #nosec G204
			cmd = exec.Command("/usr/bin/apt-get",
				append([]string{"dist-upgrade", "-d", "-o", "Debug::NoLocking=1", "-c", LastoreAptV2CommonConfPath, "--assume-no",
					"-o", fmt.Sprintf("%v=%v", "Dir::Etc::sourcelist", path),
					"-o", fmt.Sprintf("%v=%v", "Dir::Etc::SourceParts", "/dev/null")}, pkgList...)...)
		}
		logger.Infof("%v download size cmd: %v", updateType.JobType(), cmd.String())
		lines, err := utils.FilterExecOutput(cmd, time.Second*120, func(line string) bool {
			_, _, _err := parsePackageSize(line)
			return _err == nil
		})
		if err != nil && len(lines) == 0 {
			return fmt.Errorf("run:%v failed-->%v", cmd.Args, err)
		}

		if len(lines) != 0 {
			needDownloadSize, allSize, err := parsePackageSize(lines[0])
			if err != nil {
				logger.Warning(err)
				return err
			}
			*downloadSize = needDownloadSize
			*allPackageSize = allSize
		}
		return nil
	})
	if err != nil {
		logger.Warning(err)
		return SizeDownloaded, SizeDownloaded, err
	}
	logger.Debug("end QuerySourceDownloadSize duration:", time.Now().Sub(startTime))
	return *downloadSize, *allPackageSize, nil
}

// QueryPackageInstalled query whether the pkgId installed
func QueryPackageInstalled(pkgId string) bool {
	// Use Output() instead of CombinedOutput() because we only need stdout for status parsing,
	// and stderr might contain warnings that don't affect the actual package status
	out, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f", "${db:Status-Status}", "--", pkgId).Output() // #nosec G204
	if err != nil {
		return false
	}
	status := string(bytes.TrimSpace(out))
	return status == "installed"
}

// QueryPackageInstallable query whether the pkgId can be installed
func QueryPackageInstallable(pkgId string) bool {
	err := exec.Command("/usr/bin/apt-cache", "-c", LastoreAptV2CommonConfPath, "show", "--", pkgId).Run() // #nosec G204
	if err != nil {
		return false
	}

	out, err := exec.Command("/usr/bin/apt-cache", "-c", LastoreAptV2CommonConfPath, "policy", "--", pkgId).CombinedOutput() // #nosec G204
	if err != nil {
		return false
	}
	if strings.Contains(string(out), `Candidate: (none)`) {
		return false
	}
	return true
}

func QuerySourceAddSize(updateType UpdateType) (float64, error) {
	startTime := time.Now()
	addSize := new(float64)
	err := CustomSourceWrapper(updateType, func(path string, unref func()) error {
		defer func() {
			if unref != nil {
				unref()
			}
		}()
		var cmd *exec.Cmd
		if utils2.IsDir(path) {
			// #nosec G204
			cmd = exec.Command("/usr/bin/apt-get",
				[]string{"dist-upgrade", "-d", "-o", "Debug::NoLocking=1", "-c", LastoreAptV2CommonConfPath, "--assume-no",
					"-o", fmt.Sprintf("%v=%v", "Dir::Etc::sourcelist", "/dev/null"),
					"-o", fmt.Sprintf("%v=%v", "Dir::Etc::SourceParts", path)}...)
		} else {
			// #nosec G204
			cmd = exec.Command("/usr/bin/apt-get",
				[]string{"dist-upgrade", "-d", "-o", "Debug::NoLocking=1", "-c", LastoreAptV2CommonConfPath, "--assume-no",
					"-o", fmt.Sprintf("%v=%v", "Dir::Etc::sourcelist", path),
					"-o", fmt.Sprintf("%v=%v", "Dir::Etc::SourceParts", "/dev/null")}...)
		}

		lines, err := utils.FilterExecOutput(cmd, time.Second*120, func(line string) bool {
			_, _err := parseInstallAddSize(line)
			return _err == nil
		})
		if err != nil && len(lines) == 0 {
			return fmt.Errorf("run:%v failed-->%v", cmd.Args, err)
		}

		if len(lines) != 0 {
			allSize, err := parseInstallAddSize(lines[0])
			if err != nil {
				logger.Warning(err)
				return err
			}
			*addSize = allSize
		}
		return nil
	})
	if err != nil {
		logger.Warning(err)
		return SizeUnknown, err
	}
	logger.Debug("end QuerySourceDownloadSize duration:", time.Now().Sub(startTime))
	return *addSize, nil
}

// SystemArchitectures return the system package manager supported architectures
func SystemArchitectures() ([]Architecture, error) {
	foreignArchs, err := exec.Command("dpkg", "--print-foreign-architectures").Output()
	if err != nil {
		return nil, fmt.Errorf("GetSystemArchitecture failed:%v %v\n", foreignArchs, err)
	}

	arch, err := exec.Command("dpkg", "--print-architecture").Output()
	if err != nil {
		return nil, fmt.Errorf("GetSystemArchitecture failed:%v %v\n", foreignArchs, err)
	}

	var r []Architecture
	if v := Architecture(strings.TrimSpace(string(arch))); v != "" {
		r = append(r, v)
	}
	for _, a := range strings.Split(strings.TrimSpace(string(foreignArchs)), "\n") {
		if v := Architecture(a); v != "" {
			r = append(r, v)
		}
	}
	return r, nil
}

func init() {
	err := DecodeJson(path.Join(VarLibDir, "repository_info.json"), &RepoInfos)
	if err != nil {
		RepoInfos = []RepositoryInfo{}
	}
	_ = os.Setenv("DEBIAN_FRONTEND", "noninteractive")
	_ = os.Setenv("DEBIAN_PRIORITY", "critical")
	_ = os.Setenv("DEBCONF_NONINTERACTIVE_SEEN", "true")
	_ = os.Setenv("IMMUTABLE_DISABLE_REMOUNT", "true")
}

func guestBasePackageName(pkgId string) string {
	for _, sep := range []string{"-", ":", "_"} {
		index := strings.LastIndex(pkgId, sep)
		if index != -1 {
			return pkgId[:index]
		}
	}
	return pkgId
}

// see the apt code of command-line/apt-get.c:895
var __ReDownloadSize__ = regexp.MustCompile("Need to get ([0-9,.]+) ([kMGTPEZY]?)B(/[0-9,.]+)?[ ]?([kMGTPEZY]?)B? of archives")
var __unitTable__ = map[byte]float64{
	0:   1,
	'k': 1000,
	'M': 1000 * 1000,
	'G': 1000 * 1000 * 1000,
	'T': 1000 * 1000 * 1000 * 1000,
	'P': 1000 * 1000 * 1000 * 1000 * 1000,
	'E': 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
	'Z': 1000 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
	'Y': 1000 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
}

const SizeDownloaded = 0
const SizeUnknown = -1

// parsePackageSize return args[0] 当前需要下载的大小 args[1] 当前更新下载总量 args[2] error
func parsePackageSize(line string) (float64, float64, error) {
	ms := __ReDownloadSize__.FindSubmatch(([]byte)(line))
	switch len(ms) {
	case 5:
		// ms[0] 匹配的字符串
		// ms[1] 待下载大小
		// ms[2] 待下载单位(可以为空,为空时单位为B)
		// ms[3] 全部更新大小(可以为空,为空时可以认为和ms[1]相同)
		// ms[4] 全部更新单位(可以为空,为空时可以认为和ms[2]相同)
		var allDownloadSize float64
		var allDownloadUnit byte
		var needDownloadUnit byte
		needDownloadStr := strings.Replace(string(ms[1]), ",", "", -1)
		needDownloadSize, err := strconv.ParseFloat(needDownloadStr, 64)
		if err != nil {
			return SizeUnknown, SizeUnknown, fmt.Errorf("%q invalid : %v err", needDownloadStr, err)
		}
		if len(ms[2]) != 0 {
			needDownloadUnit = ms[2][0]
		}
		if len(ms[3]) > 0 {
			allDownloadStr := strings.Replace(string(ms[3]), "/", "", -1)
			allDownloadStr = strings.Replace(allDownloadStr, ",", "", -1)
			allDownloadSize, err = strconv.ParseFloat(allDownloadStr, 64)
			if err != nil {
				logger.Warning(err)
				return SizeUnknown, SizeUnknown, fmt.Errorf("%q invalid : %v err", needDownloadStr, err)
			}
			if len(ms[4]) != 0 {
				allDownloadUnit = ms[4][0]
			}
		} else {
			allDownloadSize = needDownloadSize
			allDownloadUnit = needDownloadUnit
		}
		needDownloadSize = needDownloadSize * __unitTable__[needDownloadUnit]
		allDownloadSize = allDownloadSize * __unitTable__[allDownloadUnit]
		return needDownloadSize, allDownloadSize, nil
	}
	return SizeUnknown, SizeUnknown, fmt.Errorf("%q invalid", line)
}

var __InstallAddSize__ = regexp.MustCompile("After this operation, ([0-9,.]+) ([kMGTPEZY]?)B")

func parseInstallAddSize(line string) (float64, error) {
	ms := __InstallAddSize__.FindSubmatch(([]byte)(line))
	switch len(ms) {
	case 3:
		// ms[0] 匹配的字符串
		// ms[1] 增加大小数字
		// ms[2] 增加大小单位(可以为空,为空时单位为B)
		var unit byte
		addStr := strings.Replace(string(ms[1]), ",", "", -1)
		addSize, err := strconv.ParseFloat(addStr, 64)
		if err != nil {
			return SizeUnknown, fmt.Errorf("%q invalid : %v err", addStr, err)
		}
		if len(ms[2]) != 0 {
			unit = ms[2][0]
		}

		addSize = addSize * __unitTable__[unit]
		return addSize, nil
	}
	return SizeUnknown, fmt.Errorf("%q invalid", line)
}

func CheckInstallAddSize(updateType UpdateType) bool {
	isSatisfied := false
	addSize, err := QuerySourceAddSize(updateType)
	if err != nil {
		logger.Warning(err)
	}
	logger.Debugf("add size is %v", addSize)
	content, err := exec.Command("/bin/sh", []string{
		"-c",
		"df -BK --output='avail' /usr|awk 'NR==2'",
	}...).CombinedOutput()
	if err != nil {
		logger.Warning(string(content))
	} else {
		spaceStr := strings.Replace(string(content), "K", "", -1)
		spaceStr = strings.TrimSpace(spaceStr)
		spaceNum, err := strconv.Atoi(spaceStr)
		if err != nil {
			logger.Warning(err)
		} else {
			spaceNum = spaceNum * 1000
			isSatisfied = spaceNum > int(addSize)
		}
	}
	return isSatisfied
}
