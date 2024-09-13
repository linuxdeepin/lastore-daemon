package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
)

// ar: kubuntu-23.04-desktop-amd64.iso: No space left on device
// 返回值为oup解压后的路径
func unzip(path string) (string, error) {
	cmd := exec.Command(unzipBin, "-x", path)
	dir := filepath.Join(unzipOupDir, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	cmd.Dir = dir
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return "", err
	}
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if err != nil {
		logger.Warning(outBuf.String(), errBuf.String())
		return "", errors.New(errBuf.String())
	}
	return dir, nil
}

func verify(dir string) error {
	// format验签
	cmd := exec.Command(verifyBin, "-f", filepath.Join(dir, "oup-format"), "-s", filepath.Join(dir, "oup-format_sign"))
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to verify oup-format: %v %v", outBuf.String(), errBuf.String())
	}
	// format获取
	version, err := ioutil.ReadFile(filepath.Join(dir, "oup-format"))
	if err != nil {
		return fmt.Errorf("failed to read oup-format: %v ", err)
	}
	// repo验签
	switch string(version) {
	case "1.0":
		outBuf.Reset()
		errBuf.Reset()
		cmd = exec.Command(verifyBin, "-f", filepath.Join(dir, "repo.sfs"), "-s", filepath.Join(dir, "repo.sfs_sign"))
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to verify repo.sfs: %v %v", outBuf.String(), errBuf.String())
		}
	default:
		return fmt.Errorf("can not parse this oup format version: %v", string(version))
	}
	// info验签
	outBuf.Reset()
	errBuf.Reset()
	cmd = exec.Command(verifyBin, "-f", filepath.Join(dir, "info.json"), "-s", filepath.Join(dir, "info.json_sign"))
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to verify info.json: %v %v", outBuf.String(), errBuf.String())
	}

	return nil
}
func getInfo(dir string) (OfflineRepoInfo, error) {
	content, err := ioutil.ReadFile(filepath.Join(dir, "info.json"))
	if err != nil {
		return OfflineRepoInfo{}, err
	}
	var info OfflineRepoInfo
	err = json.Unmarshal(content, &info)
	if err != nil {
		return OfflineRepoInfo{}, err
	}
	info.message = string(content)
	return info, nil
}

func systemTypeCheck(info OfflineRepoInfo) error {
	infoMap, err := updateplatform.GetOSVersionInfo(updateplatform.CacheVersion)
	if err != nil {
		logger.Warning(err)
		return err
	}
	if infoMap["EditionName"] != info.Data.SystemType {
		return errors.New("oup systemType not match EditionName")
	}
	return nil
}

func archCheck(info OfflineRepoInfo) error {
	res, err := exec.Command("dpkg", "--print-architecture").Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(res)) != info.Data.Archs {
		return errors.New("oup arch not match system arch")
	}
	return nil
}

func getOupType(info OfflineRepoInfo) (OfflineUpgradeType, error) {
	switch info.Type {
	case offlineSystem, offlineCEV, toBType:
		return info.Type, nil
	default:
		return unknownType, errors.New("")
	}
}

func mount(dir string) (string, error) {
	fsPath := filepath.Join(dir, "repo.sfs")
	hash := sha256.New()
	hash.Write([]byte(filepath.Base(dir)))
	mountDir := filepath.Join(mountFsDir, hex.EncodeToString(hash.Sum(nil)))
	err := os.MkdirAll(mountDir, 0755)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("mount", fsPath, mountDir)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to mount: %v %v", outBuf.String(), errBuf.String())
	}
	return mountDir, err
}

func isMountPoint(path string) bool {
	err := exec.Command("mountpoint", path).Run()
	return err == nil
}

// repos: 离线仓库地址列表 单个地址eg:deb [trusted=yes] file:///home/lee/patch/temp/ eagle main
func updateOfflineSourceFile(localOupRepoPaths []string) error {
	var repos []string
	for _, dir := range localOupRepoPaths {
		repos = append(repos, fmt.Sprintf("deb [trusted=yes] file://%v/ eagle main contrib non-free", dir))
	}
	return ioutil.WriteFile(system.GetCategorySourceMap()[system.OfflineUpdate], []byte(strings.Join(repos, "\n")), 0644)
}
