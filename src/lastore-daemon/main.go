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

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"internal/system"
	"internal/system/apt"
	la_utils "internal/utils"

	log "github.com/cihub/seelog"
	hhardware "github.com/jouyouyun/hardware"

	"pkg.deepin.io/dde/api/inhibit_hint"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/gettext"
	"pkg.deepin.io/lib/utils"
)

const (
	dbusServiceName = "com.deepin.lastore"
)
const (
	etcDir            = "/etc"
	osVersionFileName = "os-version"
	aptConfDir        = "/etc/apt/apt.conf.d"
	tokenConfFileName = "99lastore-token.conf"
)

func Tr(text string) string {
	return text
}

//go:generate dbusutil-gen -type Updater,Job,Manager -output dbusutil.go -import internal/system,pkg.deepin.io/lib/dbus1 updater.go job.go manager.go

func main() {
	flag.Parse()

	err := la_utils.SetSeelogger(la_utils.DefaultLogLevel, la_utils.DefaultLogFormat, la_utils.DefaultLogOutput)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	service, err := dbusutil.NewSystemService()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	log.Info("Starting lastore-daemon")
	defer log.Flush()

	hasOwner, err := service.NameHasOwner(dbusServiceName)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	if hasOwner {
		fmt.Println("another lastore-daemon running")
		return
	}

	_ = utils.UnsetEnv("LC_ALL")
	_ = utils.UnsetEnv("LANGUAGE")
	_ = utils.UnsetEnv("LC_MESSAGES")
	_ = utils.UnsetEnv("LANG")

	gettext.InitI18n()
	gettext.Textdomain("lastore-daemon")

	if os.Getenv("DBUS_STARTER_BUS_TYPE") != "" {
		_ = os.Setenv("PATH", os.Getenv("PATH")+":/bin:/sbin:/usr/bin:/usr/sbin")
	}

	b := apt.New()
	config := NewConfig(path.Join(system.VarLibDir, "config.json"))

	manager := NewManager(service, b, config)
	updater := NewUpdater(service, manager, config)
	manager.updater = updater
	err = service.Export("/com/deepin/lastore", manager, updater)
	if err != nil {
		_ = log.Error("failed to export manager and updater:", err)
		return
	}

	ihObj := inhibit_hint.New("lastore-daemon")
	ihObj.SetIcon("preferences-system")
	ihObj.SetName(Tr("Control Center"))
	err = ihObj.Export(service)
	if err != nil {
		_ = log.Warn("failed to export inhibit hint:", err)
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		_ = log.Error("failed to request name:", err)
		return
	}

	// Force notify changed at the first time
	manager.PropsMu.RLock()
	err = manager.emitPropChangedJobList(manager.JobList)
	if err != nil {
		_ = log.Warn(err)
	}
	err = manager.emitPropChangedUpgradableApps(manager.UpgradableApps)
	if err != nil {
		_ = log.Warn(err)
	}
	manager.PropsMu.RUnlock()

	log.Info("Started service at system bus")
	RegisterMonitor(manager.handleUpdateInfosChanged, system.VarLibDir, "update_infos.json")
	RegisterMonitor(updateTokenConfigFile, etcDir, osVersionFileName)
	manager.handleUpdateInfosChanged()
	time.AfterFunc(60*time.Second, func() {
		updateTokenConfigFile()
	})
	service.Wait()
}

func RegisterMonitor(handler func(), dir string, paths ...string) {
	dm := system.NewDirMonitor(dir)
	err := dm.Add(func(filePath string) {
		handler()
	}, paths...)
	if err != nil {
		_ = log.Warnf("Can't add monitor on %s: %v\n", dir, err)
	}
	err = dm.Start()
	if err != nil {
		_ = log.Warnf("Can't create monitor on %s: %v\n", dir, err)
	}
}

var _tokenUpdateMu sync.Mutex

// 更新 99lastore-token.conf 文件的内容
func updateTokenConfigFile() {
	_tokenUpdateMu.Lock()
	defer _tokenUpdateMu.Unlock()
	systemInfo, err := getSystemInfo()
	tokenPath := path.Join(aptConfDir, tokenConfFileName)
	if err != nil {
		_ = log.Warn("failed to update 99lastore-token.conf content:", err)
		return
	}
	var tokenSlice []string
	tokenSlice = append(tokenSlice, "a="+systemInfo.SystemName)
	tokenSlice = append(tokenSlice, "b="+systemInfo.ProductType)
	tokenSlice = append(tokenSlice, "c="+systemInfo.EditionName)
	tokenSlice = append(tokenSlice, "v="+systemInfo.Version)
	tokenSlice = append(tokenSlice, "i="+systemInfo.HardwareId)
	token := strings.Join(tokenSlice, ";")
	tokenContent := []byte("Acquire::SmartMirrors::Token \"" + token + "\";\n")
	err = ioutil.WriteFile(tokenPath, tokenContent, 0644)
	if err != nil {
		_ = log.Warn(err)
	}
}

type SystemInfo struct {
	SystemName  string
	ProductType string
	EditionName string
	Version     string
	HardwareId  string
}

func loadFile(filepath string) ([]string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	var lines []string
	scanner := bufio.NewScanner(bufio.NewReader(f))
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}
	if scanner.Err() != nil {
		return nil, scanner.Err()
	}

	return lines, nil
}

func getSystemInfo() (SystemInfo, error) {
	versionPath := path.Join(etcDir, osVersionFileName)
	versionLines, err := loadFile(versionPath)
	if err != nil {
		_ = log.Warn("failed to load os-version file:", err)
		return SystemInfo{}, err
	}
	mapOsVersion := make(map[string]string)
	for _, item := range versionLines {
		itemSlice := strings.SplitN(item, "=", 2)
		if len(itemSlice) < 2 {
			continue
		}
		key := strings.TrimSpace(itemSlice[0])
		value := strings.TrimSpace(itemSlice[1])
		mapOsVersion[key] = value
	}
	// 判断必要内容是否存在
	necessaryKey := []string{"SystemName", "ProductType", "EditionName", "MajorVersion", "MinorVersion", "OsBuild"}
	for _, key := range necessaryKey {
		if value := mapOsVersion[key]; value == "" {
			return SystemInfo{}, errors.New("os-version lack necessary content")
		}
	}
	systemInfo := SystemInfo{}
	systemInfo.SystemName = mapOsVersion["SystemName"]
	systemInfo.ProductType = mapOsVersion["ProductType"]
	systemInfo.EditionName = mapOsVersion["EditionName"]
	systemInfo.Version = strings.Join([]string{
		mapOsVersion["MajorVersion"],
		mapOsVersion["MinorVersion"],
		mapOsVersion["OsBuild"]},
		".")
	systemInfo.HardwareId, err = getHardwareId()
	if err != nil {
		_ = log.Warn("failed to get hardwareId:", err)
		return SystemInfo{}, err
	}
	return systemInfo, nil
}

func getHardwareId() (string, error) {
	hhardware.IncludeDiskInfo = true
	machineID, err := hhardware.GenMachineID()
	if err != nil {
		return "", err
	}
	return machineID, nil
}
