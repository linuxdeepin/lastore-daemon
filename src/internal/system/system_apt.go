/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

/*
This is system package manager need implement for porting
lastore-daemon
*/
package system

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"internal/utils"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	LastoreAptConfPath = "/var/lib/lastore/apt_v2.conf"
)

// ListPackageFile list files path contained in the packages
func ListPackageFile(packages ...string) []string {
	desktopFiles, err := utils.FilterExecOutput(
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
func GetArchivesDir() (string, error) {
	binAptConfig, _ := exec.LookPath("apt-config")
	output, err := exec.Command(binAptConfig, "-c", LastoreAptConfPath, "--format", "%f=%v%n", "dump", "Dir").Output()
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

// QueryPackageDownloadSize parsing the total size of download archives when installing
// the packages.
func QueryPackageDownloadSize(packages ...string) (float64, error) {
	if len(packages) == 0 {
		return SizeDownloaded, NotFoundError("hasn't any packages")
	}
	cmd := exec.Command("/usr/bin/apt-get",
		append([]string{"-d", "-o", "Debug::NoLocking=1", "-c", LastoreAptConfPath, "--print-uris", "--assume-no", "install", "--"}, packages...)...)

	lines, err := utils.FilterExecOutput(cmd, time.Second*10, func(line string) bool {
		_, _err := parsePackageSize(line)
		return _err == nil
	})
	if err != nil && len(lines) == 0 {
		return SizeUnknown, fmt.Errorf("Run:%v failed-->%v", cmd.Args, err)
	}

	if len(lines) != 0 {
		return parsePackageSize(lines[0])
	}
	return SizeDownloaded, nil
}

// QueryPackageInstalled query whether the pkgId installed
func QueryPackageInstalled(pkgId string) bool {
	out, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f", "${db:Status-Status}", "--", pkgId).CombinedOutput()
	if err != nil {
		return false
	}
	status := string(bytes.TrimSpace(out))
	return status == "installed"
}

// QueryPackageInstallable query whether the pkgId can be installed
func QueryPackageInstallable(pkgId string) bool {
	err := exec.Command("/usr/bin/apt-cache", "show", "--", pkgId).Run()
	if err != nil {
		return false
	}

	out, err := exec.Command("/usr/bin/apt-cache", "policy", "--", pkgId).CombinedOutput()
	if err != nil {
		return false
	}
	if strings.Contains(string(out), `Candidate: (none)`) {
		return false
	}
	return true
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

var defaultRepoInfo = RepositoryInfo{
	Name:   "desktop",
	Url:    "http://packages.deepin.com/deepin",
	Mirror: "http://cdn.packages.deepin.com/deepin",
}

func init() {
	err := DecodeJson(path.Join(VarLibDir, "repository_info.json"), &RepoInfos)
	if err != nil {
		RepoInfos = []RepositoryInfo{defaultRepoInfo}
	}
	os.Setenv("DEBIAN_FRONTEND", "noninteractive")
	os.Setenv("DEBIAN_PRIORITY", "critical")
	os.Setenv("DEBCONF_NONINTERACTIVE_SEEN", "true")
}

func DetectDefaultRepoInfo(rInfos []RepositoryInfo) RepositoryInfo {
	f, err := os.Open("/etc/apt/sources.list")
	if err != nil {
		return defaultRepoInfo
	}
	defer f.Close()

	r := bufio.NewReader(f)
	for {
		bs, _, err := r.ReadLine()
		if err != nil {
			break
		}
		line := strings.TrimLeft(string(bs), " ")
		if strings.IndexByte(line, '#') == 0 {
			continue
		}

		for _, repo := range rInfos {
			if strings.Contains(line, " "+repo.Url+" ") {
				return repo
			}
		}
	}
	return defaultRepoInfo
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
var __ReDownloadSize__ = regexp.MustCompile("Need to get ([0-9,.]+) ([kMGTPEZY]?)B(/[0-9,.]+ [kMGTPEZY]?B)? of archives")

var __unitTable__ = map[byte]float64{
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

func parsePackageSize(line string) (float64, error) {
	ms := __ReDownloadSize__.FindSubmatch(([]byte)(line))
	switch len(ms) {
	case 3, 4:
		l := strings.Replace(string(ms[1]), ",", "", -1)
		size, err := strconv.ParseFloat(l, 64)
		if err != nil {
			return SizeUnknown, fmt.Errorf("%q invalid : %v err", l, err)
		}
		if len(ms[2]) == 0 {
			return size, nil
		}
		unit := ms[2][0]
		return size * __unitTable__[unit], nil
	}
	return SizeUnknown, fmt.Errorf("%q invalid", line)
}
