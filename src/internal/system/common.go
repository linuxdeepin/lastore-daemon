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
	Id   string
	Name string
	Url  string

	NameLocale map[string]string
	Weight     int
}

var DefaultMirror = MirrorSource{
	Id:   "default",
	Url:  "http://cdn.packages.deepin.com/deepin",
	Name: "Official Mirror",
	NameLocale: map[string]string{
		"zh_CN": "官方源",
	},
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

func SystemUpgradeInfo() ([]UpgradeInfo, error) {
	var r []UpgradeInfo
	err := DecodeJson(path.Join(VarLibDir, "update_infos.json"),
		&r)
	if err != nil {
		return nil, fmt.Errorf("Invalid update_infos: %v\n", err)
	}
	return r, nil
}
