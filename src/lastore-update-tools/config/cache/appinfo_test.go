// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package cache

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

var validinfo = &AppInfo{
	Name:       "TestApp",
	Version:    "1.0.0",
	Arch:       "x86_64",
	Need:       "none",
	Filename:   "testapp.deb",
	HashSha256: "sfsafs123r",
	// HashSha256: "308a9336275b75f8400b2e12b5d1365d091342c3277a8c1dd40e408deccfc282",
	Url:      "xxxx://example.com/testapp",
	FilePath: "/media/dingh/_dde_data/home/dingh/code1/2-bug-list_start20230329/task/14_1070-system-update/tools/v2-2023-10-10/visual-add-function/deepin-system-update-tools-visual/config/cache/app",
}

var invalidinfo = &AppInfo{
	Name:       "TestApp1",
	Version:    "1.0.0",
	Arch:       "x86_64",
	Need:       "none",
	Filename:   "testapp1.deb",
	HashSha256: "abcdef12345678",
	Url:        "xxxx://example.com/testapp",
	FilePath:   "/media/dingh/_dde_data/home/dingh/code1/2-bug-list_start20230329/task/14_1070-system-update/tools/v2-2023-10-10/visual-add-function/deepin-system-update-tools-visual/config/cache/app",
}

func TestAppInfo(t *testing.T) {
	// 创建一个 AppInfo 实例
	// 打印整个 info 对象
	fmt.Printf("dingh-test-start\n")
	fmt.Printf("%+v\n", validinfo)

	//生成json文件
	jsonBytes, err := json.Marshal(*validinfo)
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile("/tmp/appinfo.json", jsonBytes, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func TestCompareVerion(t *testing.T) {
	// fmt.Printf("%+v %+v",info.Name, info.Version)
	t.Run("ValidVersion", func(t *testing.T) {
		if err := validinfo.CompareVerion("TestApp", "1.0.0"); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("InValidVersion", func(t *testing.T) {
		if err := invalidinfo.CompareVerion("TestApp", "2.0.0"); err == nil {
			t.Errorf("expected error:%v, got nil", err)
		}
	})
}

func TestCheckFileExist(t *testing.T) {
	t.Run("ExistFile", func(t *testing.T) {
		if err := fs.CheckFileExistState(invalidinfo.FilePath + "/" + invalidinfo.Filename); err != nil {
			t.SkipNow()
		}
		if err := validinfo.CheckFileExist(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("InExistFile", func(t *testing.T) {
		if err := fs.CheckFileExistState(invalidinfo.FilePath + "/" + invalidinfo.Filename); err != nil {
			t.SkipNow()
		}
		if err := invalidinfo.CheckFileExist(); err == nil {
			t.Errorf("expected error:%v, got nil", err)
		}
	})
}

func TestCompareHashSha256(t *testing.T) {
	t.Run("InValidHashSha256", func(t *testing.T) {
		fmt.Printf("%+v\n", validinfo.HashSha256)
		file := validinfo.FilePath + "/" + validinfo.Filename
		hs256, _ := fs.FileHashSha256(file)
		fmt.Printf("hs256:%v\n", hs256)
		if err := validinfo.CompareHashSha256(); err == nil {
			t.Errorf("expected error:%v, got nil", err)
		}
	})

	t.Run("ValidHashSha256", func(t *testing.T) {
		fmt.Printf("%+v\n", validinfo.HashSha256)
		file := validinfo.FilePath + "/" + validinfo.Filename
		if err := fs.CheckFileExistState(file); err != nil {
			t.SkipNow()
		}
		hs256, _ := fs.FileHashSha256(file)
		validinfo.HashSha256 = hs256
		fmt.Printf("hs256:%v\n", validinfo.HashSha256)
		if err := validinfo.CompareHashSha256(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}
