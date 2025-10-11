// SPDX-FileCopyrightText: 2018 - 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dut

import (
	"encoding/json"
	"os"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"

	"github.com/linuxdeepin/go-lib/utils"
)

type DutSystem struct {
	apt.APTSystem
}

func NewSystem(nonUnknownList []string, otherList []string, incrementalUpdate bool) system.System {
	logger.Info("using dut for update...")
	aptImpl := apt.New(nonUnknownList, otherList, incrementalUpdate)
	if !utils.IsFileExist(system.PlatFormSourceFile) {
		file, err := os.Create(system.PlatFormSourceFile)
		if err != nil {
			logger.Warning("creating file:", err)
		}
		defer file.Close()
	}
	return &DutSystem{
		APTSystem: aptImpl,
	}
}

func OptionToArgs(options map[string]string) []string {
	var args []string
	for key, value := range options { // dut 命令执行参数
		args = append(args, key)
		args = append(args, value)
	}
	return args
}

func (p *DutSystem) UpdateSource(jobId string, environ map[string]string, args map[string]string) error {
	// 依赖错误放到后面检查
	// err := checkSystemDependsError()
	// if err != nil {
	// 	return err
	// }
	return p.APTSystem.UpdateSource(jobId, environ, args)
}

func (p *DutSystem) DistUpgrade(jobId string, packages []string, environ map[string]string, args map[string]string) error {
	err := checkSystemDependsError(p.Indicator)
	if err != nil {
		return err
	}
	return p.APTSystem.DistUpgrade(jobId, packages, environ, args)
}

func (p *DutSystem) FixError(jobId string, errType string, environ map[string]string, args map[string]string) error {
	return p.APTSystem.FixError(jobId, errType, environ, args)
}

func (p *DutSystem) OsBackup(jobId string) error {
	return p.APTSystem.OsBackup(jobId)
}

func (p *DutSystem) CheckSystem(jobId string, checkType string, environ map[string]string, options map[string]string) error {
	// environ parameter is ignored here
	fn := system.NewFunction(jobId, p.Indicator, func() error {
		// only postCheck can be handled here, checkType is ignored
		systemErr := CheckSystem(PostCheck, options)
		if systemErr != nil {
			return systemErr
		}
		return nil
	})
	return fn.Start()
}

func checkSystemDependsError(indicator system.Indicator) error {
	err := apt.CheckPkgSystemError(false, indicator)
	if err != nil {
		logger.Warningf("apt-get check failed:%v", err)
		return err
	}
	return nil
}

type CheckType uint

const (
	PreCheck  CheckType = 0
	MidCheck  CheckType = 1
	PostCheck CheckType = 2
)

func (t CheckType) String() string {
	switch t {
	case PreCheck:
		return "precheck"
	case MidCheck:
		return "midcheck"
	case PostCheck:
		return "postcheck"
	}
	return ""
}

type RepoInfo struct {
	Name       string
	FilePath   string
	HashSha256 string
}

type metaInfo struct {
	PkgDebPath string
	CoreList   []system.PackageInfo
	ReposInfo  []RepoInfo `json:"RepoInfo"`
	UUID       string
	Time       string
}

const (
	strict      = "strict"
	skipState   = "skipstate"
	skipVersion = "skipversion"
	exist       = "exist"
)

// GenDutMetaFile metaPath为DutOnlineMetaConfPath或DutOfflineMetaConfPath
func GenDutMetaFile(metaPath, debPath string,
	coreMap map[string]system.PackageInfo, repoInfo []RepoInfo) (string, error) {
	meta := metaInfo{
		PkgDebPath: debPath,
		UUID:       utils.GenUuid(),
		Time:       time.Now().String(),
		ReposInfo:  repoInfo,
		CoreList:   genPkgList(coreMap),
	}
	content, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(metaPath, content, 0644)
	if err != nil {
		return "", err
	}
	return meta.UUID, nil
}

func genPkgList(pkgMap map[string]system.PackageInfo) []system.PackageInfo {
	var list []system.PackageInfo
	for _, v := range pkgMap {
		info := system.PackageInfo{
			Name:    v.Name,
			Version: v.Version,
			Need:    skipVersion,
		}
		list = append(list, info)
	}
	return list
}
