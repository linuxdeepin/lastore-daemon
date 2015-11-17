package main

import (
	"bufio"
	"fmt"
	log "github.com/cihub/seelog"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
)

type DesktopInfo struct {
	FilePath string
	Package  string
	Icon     string
	Exec     string
}

// GenerateDesktopIndexes 生成 desktop 相关的查询表
// 1. desktop --> icon
// 2. desktop --> exec
// 3. desktop --> package
func GenerateDesktopIndexes(scanDirectories []string, outputDir string) error {
	os.MkdirAll(outputDir, 0755)
	packageIndex, installTimeIndex := loadPackageInfos()

	var desktopPaths []string
	for _, dir := range scanDirectories {
		fs, err := ioutil.ReadDir(dir)
		if err != nil {
			log.Warnf("GenerateDesktopIndexes :%v\n", err)
			continue
		}
		for _, finfo := range fs {
			name := finfo.Name()
			if strings.HasSuffix(name, ".desktop") {
				desktopPaths = append(desktopPaths, path.Join(dir, finfo.Name()))
			}
		}
	}

	var dinfos []DesktopInfo
	for _, dPath := range desktopPaths {
		info := ParseDesktopInfo(packageIndex, dPath)
		if info != nil {
			dinfos = append(dinfos, *info)
		}
	}
	writeDesktopExecIndex(dinfos, path.Join(outputDir, "package_exec.json"))
	writeDesktopIconIndex(dinfos, path.Join(outputDir, "package_icon.json"))
	writeDesktopPackage(dinfos, path.Join(outputDir, "desktop_package.json"))
	writeData(path.Join(outputDir, "package_installedTime.json"), installTimeIndex)

	return nil
}

var iconRegexp = regexp.MustCompile(`Icon=(.+)`)
var execRegexp = regexp.MustCompile("Exec=(.+)")

// ParseDesktopInfo 根据文件列表返回分析结果
func ParseDesktopInfo(packagesIndex map[string]string, dPath string) *DesktopInfo {
	f, err := os.Open(dPath)
	if err != nil {
		fmt.Println("ParseDesktopInfo:", err)
		return nil
	}
	defer f.Close()

	buf := bufio.NewReader(f)

	var icon, exec string

	var line string
	for err == nil {
		line, err = buf.ReadString('\n')
		rr := iconRegexp.FindStringSubmatch(line)
		if len(rr) == 2 {
			icon = rr[1]
		}
		rr = execRegexp.FindStringSubmatch(line)
		if len(rr) == 2 {
			exec = rr[1]
		}
		if icon != "" && exec != "" {
			break
		}
	}

	info := DesktopInfo{
		FilePath: dPath,
		Package:  packagesIndex[path.Base(dPath)],
		Icon:     icon,
		Exec:     exec,
	}

	return &info
}

func getDesktopFilePaths(listFilePath string) []string {
	f, err := os.Open(listFilePath)
	if err != nil {
		fmt.Println("getDesktopFilePaths:", err)
		return nil
	}
	defer f.Close()

	var r []string

	var line string
	buf := bufio.NewReader(f)
	for err == nil {
		line, err = buf.ReadString('\n')
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ".desktop") {
			r = append(r, line)
		}
	}
	return r
}

func getPackageName(name string) string {
	if len(name) <= 5 {
		return name
	}
	baseName := name[:len(name)-5]

	ns := strings.SplitN(baseName, ":", -1)
	if len(ns) != 0 {
		return ns[0]
	}
	return name
}

func loadPackageInfos() (map[string]string, map[string]int64) {
	var r = make(map[string]string)
	var t = make(map[string]int64)

	fs, err := ioutil.ReadDir("/var/lib/dpkg/info")
	if err != nil {
		log.Warnf("loadPackageInfos :%v\n", err)
		return r, t
	}

	for _, finfo := range fs {
		name := finfo.Name()
		if strings.HasSuffix(name, ".list") {
			packageName := getPackageName(name)
			desktopFiles := getDesktopFilePaths(path.Join("/var/lib/dpkg/info", name))
			if len(desktopFiles) == 0 {
				continue
			}
			for _, f := range desktopFiles {
				r[f] = packageName
				r[path.Base(f)] = packageName
			}
			t[packageName] = finfo.ModTime().Unix()
		}
	}
	return r, t
}

func writeDesktopExecIndex(infos []DesktopInfo, fpath string) {
	r := make(map[string]string)
	for _, info := range infos {
		r[info.Package] = info.Exec
	}
	writeData(fpath, r)
}

func writeDesktopIconIndex(infos []DesktopInfo, fpath string) {
	r := make(map[string]string)
	for _, info := range infos {
		r[info.Package] = info.Icon
	}
	writeData(fpath, r)
}

func writeDesktopPackage(infos []DesktopInfo, fpath string) {
	r := make(map[string]string)
	for _, info := range infos {
		r[path.Base(info.FilePath)] = info.Package
	}
	writeData(fpath, r)
}
