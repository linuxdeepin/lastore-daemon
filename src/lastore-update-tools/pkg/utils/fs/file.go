// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func CheckFileExistState(filepath string) error {
	// 使用 os.Stat 函数查询文件信息
	_, err := os.Stat(filepath)

	// 检查错误类型来确定文件是否存在
	if os.IsNotExist(err) {
		return fmt.Errorf("file %s does not exist", filepath)
	} else if err == nil {
		return nil
	} else {
		return err
	}
}

func CreateFile(fileName string) (*os.File, error) {
	absPath := filepath.Dir(fileName)
	if absPath == "" {
		return nil, errors.New("not found base name")
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		err = os.MkdirAll(absPath, 0755)
		if err != nil {
			return nil, err
		}
	}
	out, err := os.Create(fileName)
	return out, err
}

func ReadMode(fileName string) (os.FileMode, error) {
	info, err := os.Stat(fileName)
	if err != nil {
		return 0, err
	}
	return info.Mode(), nil
}

func CreateDirMode(dir string, mode os.FileMode) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, mode)
	}
	return nil
}
