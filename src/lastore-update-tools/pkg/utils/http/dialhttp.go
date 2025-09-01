// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package http

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

// DialUrlHttpGet
func DialUrlHttpGet(url string, timeout int) error {
	client := http.Client{
		Timeout: time.Second * time.Duration(timeout),
	}
	_, err := client.Get(url)
	if err != nil {
		return err
	} else {
		return nil
	}
}

// DownloadFileHttpGet
/*
* DownloadFileHttpGet
* return
* string filename
* string filepath
* error err
 */
func DownloadFileHttpGet(url string, filepath string, timeout int) (string, string, error) {
	client := http.Client{
		Timeout: time.Second * time.Duration(timeout),
	}
	response, err := client.Get(url)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to download: %s", response.Status)
	}

	// Extract the file name from the URL
	fileName := path.Base(response.Request.URL.String())
	if filepath == "" {
		filepath, err := os.Getwd()
		if err != nil {
			filepath = "./" + fileName
		} else {
			filepath = filepath + "/" + fileName
		}

	} else {
		filepath = filepath + "/" + fileName
	}
	out, err2 := fs.CreateFile(filepath)
	if err2 != nil {
		return "", "", err2
	}

	defer out.Close()

	// Read the file content
	if _, err := io.Copy(out, response.Body); err != nil {
		return "", "", err
	}

	return fileName, filepath, nil
}
