// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckFileExistState(t *testing.T) {
	file, err := ioutil.TempFile("/tmp/", "sha256_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	file.WriteString("1")

	file.Close()

	if rs, err := filepath.Glob("/tmp/sha256_*"); err != nil || len(rs) <= 0 {
		t.Error("failed: ", err, file.Name())
	} else {
		t.Logf("rs:%v", rs)

		if err := CheckFileExistState(rs[0]); err != nil {
			t.Error("failed: ", err, rs[0])
		}
	}
}
