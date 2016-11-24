/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
)

type MirrorSource struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Url  string `json:"url"`

	NameLocale map[string]string `json:"name_locale"`
	Weight     int               `json:"weight"`
}

var RepoInfos []RepositoryInfo

type RepositoryInfo struct {
	Name   string `json:"name"`
	Url    string `json:"url"`
	Mirror string `json:"mirror"`
}

func DecodeJson(fpath string, d interface{}) error {
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewDecoder(f).Decode(&d)
}

func EncodeJson(fpath string, d interface{}) error {
	f, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(d)
}

func NormalFileExists(fpath string) bool {
	info, err := os.Stat(fpath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	return true
}

func SystemUpgradeInfo() ([]UpgradeInfo, error) {
	info_path := path.Join(VarLibDir, "update_infos.json")
	if !NormalFileExists(info_path) {
		return nil, NotFoundError
	}
	var r []UpgradeInfo
	err := DecodeJson(info_path, &r)
	if err != nil {
		return nil, fmt.Errorf("Invalid update_infos: %v\n", err)
	}
	return r, nil
}
