// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package cache

import (
	"fmt"
	"os"
	"reflect"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

// strict 严格匹配
// skipstate 忽略状态
// skipversion 忽略版本
// exist 存在即可

const (
	DEEPIN_SYSTEM_APPINFO_NEED_EXIST               = "exist"
	DEEPIN_SYSTEM_APPINFO_NEED_EXIST_VERSION       = "skipstate"
	DEEPIN_SYSTEM_APPINFO_NEED_EXIST_STATE         = "skipversion"
	DEEPIN_SYSTEM_APPINFO_NEED_EXIST_VERSION_STATE = "strict"
)

// AppInfo 数据结构体
type AppInfo struct {
	Name          string `json:"Name" yaml:"Name"`                                       // name
	Version       string `json:"Version,omitempty" yaml:"Version,omitempty"`             // version
	Arch          string `json:"Arch,omitempty" yaml:"Arch,omitempty"`                   // arch
	Need          string `json:"Need,omitempty" yaml:"Need,omitempty"`                   // need
	Filename      string `json:"FileName,omitempty" yaml:"FileName,omitempty"`           // filename
	HashSha1      string `json:"SHA1,omitempty" yaml:"SHA1,omitempty"`                   // sha1
	HashSha256    string `json:"Sha256,omitempty" yaml:"Sha256,omitempty"`               // Sha256
	Url           string `json:"Url,omitempty" yaml:"Url,omitempty"`                     // url
	FilePath      string `json:"FilePath,omitempty" yaml:"FilePath,omitempty"`           // FilePath
	InstalledSize int    `json:"InstalledSize,omitempty" yaml:"InstalledSize,omitempty"` // InstalledSize
	DebSize       int    `json:"DebSize,omitempty" yaml:"DebSize,omitempty"`             // DebSize
}

// check app info
func (ts *AppInfo) Verify() error {
	if ts.Name == "" {
		return fmt.Errorf("Name")
	}
	if ts.Version == "" {
		return fmt.Errorf("Version")
	}
	if ts.Filename == "" {
		return fmt.Errorf("Filename")
	}
	if ts.HashSha256 == "" {
		return fmt.Errorf("HashSha256")
	}
	if ts.InstalledSize < 0 {
		return fmt.Errorf("InstalledSize")
	}
	if ts.DebSize < 0 {
		return fmt.Errorf("DebSize")
	}
	return nil
}

func (ts *AppInfo) Merge(rightAppInfo AppInfo) error {

	rightValueList := reflect.ValueOf(rightAppInfo)
	leftValueList := reflect.ValueOf(ts).Elem()

	for i := 0; i < leftValueList.NumField(); i++ {
		leftField := leftValueList.Field(i)
		rightField := rightValueList.FieldByName(leftValueList.Type().Field(i).Name)

		if rightField.IsValid() && rightField.Interface() != "" && !reflect.DeepEqual(rightField, leftField) {
			//fmt.Printf("rightField: %+v\n", rightField.Interface())
			leftField.Set(rightField)
		}
	}

	return nil
}

type AppTinyInfo struct {
	Name    string   `json:"Name" yaml:"Name"`                           // name
	Version string   `json:"Version,omitempty" yaml:"Version,omitempty"` // version
	State   PkgState `json:"state" yaml:"state"`                         // state
}

func (ts *AppInfo) CompareVerion(pkgName, pkgVersion string) error {
	if ts.Name == pkgName && ts.Version == pkgVersion {
		return nil
	}
	return fmt.Errorf("appinfo compare version faild")
}

func (ts *AppInfo) CheckFileExist() error {
	file := ts.FilePath + "/" + ts.Filename
	if _, err := os.Stat(file); err != nil {
		return fmt.Errorf("appinfo check file %s exist error:%+v", file, err)
	}
	return nil
}

func (ts *AppInfo) CompareHashSha256() error {
	file := ts.FilePath + "/" + ts.Filename
	if err := fs.CheckFileHashSha256(file, ts.HashSha256); err != nil {
		return fmt.Errorf("appinfo check file %s Hashsha256 error: %+v", file, err)
	}
	return nil
}
