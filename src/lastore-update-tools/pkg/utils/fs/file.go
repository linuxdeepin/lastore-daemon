// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fs

import (
	"fmt"
	"os"
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
