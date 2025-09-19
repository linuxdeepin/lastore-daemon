// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fs

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestFileHashSha256(t *testing.T) {
	oneFileSha256 := "6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b"
	file, err := ioutil.TempFile("/tmp/", "sha256_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	file.WriteString("1")

	file.Close()

	if fileSha256, err := FileHashSha256(file.Name()); fileSha256 != oneFileSha256 || err != nil {
		t.Error("failed: ", err, file.Name())
	}
}

func TestGetFileSha1(t *testing.T) {
	oneFileSha256 := "356a192b7913b04c54574d18c28d46e6395428ab"
	file, err := ioutil.TempFile("/tmp/", "sha256_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	file.WriteString("1")

	file.Close()

	if fileSha256, err := FileHashSha1(file.Name()); fileSha256 != oneFileSha256 || err != nil {
		t.Error("failed: ", err, file.Name())
	}
}
