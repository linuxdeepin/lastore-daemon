// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

	"github.com/linuxdeepin/go-lib/log"
	debVersion "pault.ag/go/debian/version"
)

const maxElapsed = time.Hour * 24 * 6 // 6 days

var (
	binDpkg      string
	binDpkgQuery string
	binDpkgDeb   string
	binAptCache  string

	logger = log.NewLogger("cmd/lastore-apt-clean")
)

func mustGetBin(name string) string {
	file, err := exec.LookPath(name)
	if err != nil {
		logger.Fatal(err)
	}
	return file
}

var options struct {
	forceDelete bool
	printJSON   bool
	incrementalUpdate bool
}

func init() {
	flag.BoolVar(&options.forceDelete, "force-delete", false, "force delete deb files")
	flag.BoolVar(&options.printJSON, "print-json", false,
		"Print information about files that can be safely deleted, in json format")
	flag.BoolVar(&options.incrementalUpdate, "incremental-update", false, "whether to enable incremental update")
	_ = os.Setenv("LC_ALL", "C")
}

func findBins() {
	binDpkg = mustGetBin("dpkg")
	binDpkgQuery = mustGetBin("dpkg-query")
	binDpkgDeb = mustGetBin("dpkg-deb")
	binAptCache = mustGetBin("apt-cache")
}

var _archivesDirInfos []*archivesDirInfo

func main() {
	flag.Parse()
	if options.printJSON {
		// 让 logger 不在标准输出打印其他内容，只打印在系统日志中。
		logger.RemoveBackendConsole()
	}
	findBins()

	// 如果是增量更新，则调用deepin-immutable-ctl upgrade cleanup命令清理immutable系统的缓存deb包和ostree包分支
	if options.incrementalUpdate {
		err := exec.Command("deepin-immutable-ctl", "upgrade", "clean").Run()
		if err != nil {
			logger.Debugf("failed to clean upgrade cache: %v", err)
		}
	}

	appendArchivesDirInfos(system.LastoreAptV2CommonConfPath) // 将lastore缓存路径/var/cache/lastore/archives添加
	appendArchivesDirInfos(system.LastoreAptOrgConfPath)      // 将默认缓存路径/var/cache/apt/archives添加

	archivesInfos := &archivesInfos{
		Files:     make(map[string][]*archiveInfo),
		TotalSize: 0,
	}
	for _, dirInfo := range _archivesDirInfos {
		logger.Info("dirInfo: %v", dirInfo)
		var archivesInfo *archivesInfo
		if options.printJSON {
			archivesInfo = newArchivesInfo(dirInfo.archivesDir)
		}

		fileInfoList, err := os.ReadDir(dirInfo.archivesDir)
		if err != nil {
			logger.Fatal(err)
		}

		cache, err := loadPkgStatusVersion()
		if err != nil {
			logger.Fatal(err)
		}

		// var testAgainDebInfoList []*debInfo

		for _, entry := range fileInfoList {
			logger.Info("entry: %v", entry)
			fileInfo, err := entry.Info()
			if err != nil {
				logger.Fatal(err)
			}
			if fileInfo.IsDir() {
				continue
			}

			if filepath.Ext(fileInfo.Name()) != ".deb" {
				continue
			}

			logger.Debug("> ", fileInfo.Name())
			var delPolicy DeletePolicy = DeleteExpired
			filename := filepath.Join(dirInfo.archivesDir, fileInfo.Name())
			debInfo, err := getDebInfo(filename)
			if err != nil {
				delPolicy = DeleteImmediately
			} else {
				logger.Debugf("debInfo: %#v\n", debInfo)
				// var testAgain bool
				delPolicy, _ = shouldDelete(debInfo, cache)
				// if testAgain {
				// 	// 需要更多地判断
				// 	debInfo.fileInfo = fileInfo
				// 	testAgainDebInfoList = append(testAgainDebInfoList, debInfo)
				// 	continue
				// }
			}
			actWithPolicy(delPolicy, fileInfo, filename, archivesInfo)
		}

		// 更新治理管控之后，不一定从本地仓库更新，所以不能在通过这种方式获取备选版本和校验,防止包被误删
		// t := time.Now()
		// err = loadCandidateVersions(testAgainDebInfoList, dirInfo.configPath)
		// logger.Debug("loadCandidateVersions cost:", time.Since(t))
		// if err != nil {
		// 	logger.Fatal("load candidate versions failed:", err)
		// }

		// for _, info := range testAgainDebInfoList {
		// 	logger.Debug(">> ", info.fileInfo.Name())
		// 	delPolicy := shouldDeleteTestAgain(info)
		// 	actWithPolicy(delPolicy, info.fileInfo, info.filename, archivesInfo)
		// }
		if archivesInfo != nil {
			archivesInfos.Files[archivesInfo.dir] = archivesInfo.Files
			archivesInfos.TotalSize += archivesInfo.TotalSize
		}
	}

	data, err := json.Marshal(archivesInfos)
	if err != nil {
		logger.Fatal(err)
	}
	_, err = os.Stdout.Write(data)
	if err != nil {
		logger.Fatal(err)
	}

}

type archivesInfo struct {
	dir       string
	Files     []*archiveInfo
	TotalSize uint64
}

type archivesInfos struct {
	Files     map[string][]*archiveInfo `json:"files"`
	TotalSize uint64                    `json:"total"`
}

func newArchivesInfo(dir string) *archivesInfo {
	return &archivesInfo{
		dir: dir,
	}
}

func (ai *archivesInfo) addFileInfo(fileInfo os.FileInfo) {
	info := &archiveInfo{
		Name: fileInfo.Name(),
		Size: fileInfo.Size(),
	}
	ai.Files = append(ai.Files, info)
	ai.TotalSize += uint64(info.Size)
}

type archiveInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func actWithPolicy(deletePolicy DeletePolicy, fileInfo os.FileInfo, filename string, archivesInfo *archivesInfo) {
	needDelete := false
	switch deletePolicy {
	case DeleteImmediately:
		needDelete = true
	case DeleteExpired:
		if options.forceDelete {
			needDelete = true
		} else {
			debChangeTime := getChangeTime(fileInfo)
			if time.Since(debChangeTime) > maxElapsed {
				needDelete = true
			} else {
				logger.Debug("delete later")
			}
		}
	case Keep:
		if options.forceDelete {
			needDelete = true
		} else {
			logger.Debug("keep")
		}
	}
	if needDelete {
		logger.Debug("delete", fileInfo.Name())
		if archivesInfo != nil {
			// 统计信息模式
			archivesInfo.addFileInfo(fileInfo)
		} else {
			deleteDeb(filename)
		}
	} else {
		logger.Debug("do not delete", fileInfo.Name())
	}
}

type DeletePolicy uint

const (
	DeleteExpired = iota
	DeleteImmediately
	Keep
)

func shouldDeleteTestAgain(debInfo *debInfo) DeletePolicy {
	candidateVersion := getCandidateVersion(debInfo)
	logger.Debug("candidate version:", candidateVersion)
	if candidateVersion == "" {
		return DeleteExpired
	}

	if candidateVersion != debInfo.version {
		logger.Debug("not the candidate version")
		return DeleteImmediately
	}
	return Keep
}

func shouldDelete(debInfo *debInfo, cache map[string]statusVersion) (delPolicy DeletePolicy, testAgain bool) {
	statusVersion, ok := cache[debInfo.pkgArch()]
	if !ok {
		// deb包是还没安装过的
		return DeleteExpired, true
	}
	logger.Debugf("current status: %q, version: %q\n", statusVersion.status, statusVersion.version)
	if len(statusVersion.status) > 0 {
		desiredAction := statusVersion.status[0]
		switch desiredAction {
		case 'i':
			// i - install
			if compareVersionsGt(debInfo.version, statusVersion.version) {
				logger.Debug("deb version great then installed version")
				return DeleteExpired, true
			}
			return DeleteImmediately, false

		case 'r', 'p', 'h':
			// r - remove
			// p - purge
			// h - hold
			return DeleteImmediately, false
		default:
			// u - unknown
			return DeleteExpired, false
		}
	}
	return DeleteExpired, false
}

type debInfo struct {
	pkg      string
	version  string
	arch     string
	fileInfo os.FileInfo
	filename string
}

func (di *debInfo) pkgArch() string {
	return di.pkg + ":" + di.arch
}

func getControlField(line []byte, key []byte) (string, error) {
	if bytes.HasPrefix(line, key) {
		return string(line[len(key):]), nil
	}
	return "", fmt.Errorf("failed to get control field %s", key[:len(key)-2])
}

func getDebInfo(filename string) (*debInfo, error) {
	const (
		fieldPkg  = "Package"
		fieldVer  = "Version"
		fieldArch = "Architecture"
		sep       = ": "
	)

	output, err := exec.Command(binDpkgDeb, "-f", "--", filename,
		fieldPkg, fieldVer, fieldArch).Output() // #nosec G204
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(output, []byte{'\n'})
	if len(lines) < 3 {
		return nil, errors.New("getDebInfo len(lines) < 3")
	}

	name, err := getControlField(lines[0], []byte(fieldPkg+sep))
	if err != nil {
		return nil, err
	}
	version, err := getControlField(lines[1], []byte(fieldVer+sep))
	if err != nil {
		return nil, err
	}
	arch, err := getControlField(lines[2], []byte(fieldArch+sep))
	if err != nil {
		return nil, err
	}
	return &debInfo{
		pkg:      name,
		version:  version,
		arch:     arch,
		filename: filename,
	}, nil
}

type statusVersion struct {
	status  string
	version string
}

func loadPkgStatusVersion() (map[string]statusVersion, error) {
	out, err := exec.Command(binDpkgQuery, "-f", "${Package}:${Architecture} ${db:Status-Abbrev} ${Version}\n", "-W").Output() // #nosec G204
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(out)
	scanner := bufio.NewScanner(reader)
	result := make(map[string]statusVersion)
	for scanner.Scan() {
		line := scanner.Bytes()
		fields := bytes.Fields(line)
		if len(fields) != 3 {
			continue
		}
		pkg := string(fields[0]) // 包含包名和架构，比如 bash:amd64
		status := string(fields[1])
		version := string(fields[2])
		result[pkg] = statusVersion{
			status:  status,
			version: version,
		}
	}
	err = scanner.Err()
	if err != nil {
		return nil, err
	}
	return result, nil
}

var _candidateCache = make(map[string]string)

func getCandidateVersion(info *debInfo) string {
	if info.arch == "all" {
		return _candidateCache[info.pkg]
	}

	// 优先尝试包名加架构
	ver, ok := _candidateCache[info.pkgArch()]
	if ok {
		return ver
	}

	// 而后尝试只有包名
	return _candidateCache[info.pkg]
}

func parseAptCachePolicyOutput(r io.Reader) map[string]string {
	scanner := bufio.NewScanner(r)
	var pkg string
	var candidate string
	result := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) >= 2 {
			if line[0] != ' ' && // 开头不是空格
				line[len(line)-1] == ':' /* 末尾是 : */ {

				// 获取包名
				pkg = string(line[:len(line)-1])

			} else if line[0] == ' ' && // 开头是空格
				bytes.Contains(line, []byte("Candidate:")) {

				// 获取候选版本
				idx := bytes.IndexByte(line, ':')
				candidate = string(bytes.TrimSpace(line[idx+1:]))

				if pkg != "" && candidate != "" {
					result[pkg] = candidate
				}
				pkg = ""
				candidate = ""
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Warning("parseAptCachePolicyOutput scanner err:", err)
	}
	return result
}

func loadCandidateVersions(debInfoList []*debInfo, configPath string) error {
	args := []string{binAptCache, "-c", configPath, "policy", "--"}

	var buf bytes.Buffer
	for _, info := range debInfoList {
		buf.WriteString(info.pkgArch())
		buf.WriteByte('\n')
	}

	// NOTE: 使用 xargs 命令来自动处理传给 apt-cache 命令参数过多的情况。
	cmd := exec.Command("xargs", args...) // #nosec G204
	cmd.Stdin = &buf
	stdOutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	result := parseAptCachePolicyOutput(stdOutPipe)

	err = cmd.Wait()
	if err != nil {
		return err
	}

	_candidateCache = result
	return nil
}

func compareVersionsGt(ver1, ver2 string) bool {
	gt, err := compareVersionsGtFast(ver1, ver2)
	if err != nil {
		return compareVersionsGtDpkg(ver1, ver2)
	}
	return gt
}

func compareVersionsGtFast(ver1, ver2 string) (bool, error) {
	v1, err := debVersion.Parse(ver1)
	if err != nil {
		return false, err
	}
	v2, err := debVersion.Parse(ver2)
	if err != nil {
		return false, err
	}
	return debVersion.Compare(v1, v2) > 0, nil
}

func compareVersionsGtDpkg(ver1, ver2 string) bool {
	err := exec.Command(binDpkg, "--compare-versions", "--", ver1, "gt", ver2).Run() // #nosec G204
	return err == nil
}

// getChangeTime get time when file status was last changed.
func getChangeTime(fileInfo os.FileInfo) time.Time {
	stat := fileInfo.Sys().(*syscall.Stat_t)
	// NOTE: 需要保留 int64() 显示转换，因为有些架构的 stat.Ctim 的 Sec 和 Nsc 的类型不是 int64。
	return time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
}

func deleteDeb(filename string) {
	err := os.Remove(filename)
	if err != nil {
		logger.Warning("deleteDeb error:", err)
	}
}

type archivesDirInfo struct {
	archivesDir string
	configPath  string
}

func appendArchivesDirInfos(confPath string) {
	archivesDir, err := system.GetArchivesDir(confPath)
	if err != nil {
		logger.Warning(err)
		return
	}
	logger.Debug("archives dir:", archivesDir)
	_archivesDirInfos = append(_archivesDirInfos, &archivesDirInfo{
		archivesDir: archivesDir,
		configPath:  confPath,
	})
}
