// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fs

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"

	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("lastore/update-tools/utils/fs")

func FileHashSha256(filename string) (string, error) {

	hasher := sha256.New()
	s, err := ioutil.ReadFile(filename)
	if err != nil {

		return "", err
	}
	_, err = hasher.Write(s)
	if err != nil {

		return "", err
	}

	sha256Sum := hex.EncodeToString(hasher.Sum(nil))
	s = nil

	return sha256Sum, nil
}

func FileHashSha1(filename string) (string, error) {

	hasher := sha1.New()
	s, err := ioutil.ReadFile(filename)
	if err != nil {

		return "", err
	}
	_, err = hasher.Write(s)
	if err != nil {

		return "", err
	}

	sha256Sum := hex.EncodeToString(hasher.Sum(nil))

	return sha256Sum, nil
}

func CheckFileHashSha1(filename string, hash string) error {
	if sha1, err := FileHashSha1(filename); err == nil {
		if sha1 == hash {
			return nil
		} else {
			return fmt.Errorf("error checking file hash")
		}
	} else {
		return err
	}
}

func CheckFileHashSha256(filename string, hash string) error {
	if sha256, err := FileHashSha256(filename); err == nil {
		if sha256 == hash {
			return nil
		} else {
			return fmt.Errorf("error checking file hash")
		}
	} else {
		return err
	}
}

func CheckRepoInfoHashSha256(filename string, hash string) error {
	if sha256, err := FileHashSha256(filename); err == nil {
		if sha256 == hash {
			return nil
		} else {
			// Only log the error, do not return an error
			logger.Warning("error checking repofile hash")
			return nil
		}
	} else {
		return err
	}
}
