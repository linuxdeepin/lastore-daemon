// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"encoding/base64"
	"encoding/json"

	. "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"

	"github.com/codegangsta/cli"
)

var CMDPostUpgrade = cli.Command{
	Name:   "postupgrade",
	Usage:  `post system upgrade message`,
	Action: MainPostUpgrade,
}

type upgradePostContent struct {
	SerialNumber    string   `json:"serialNumber"`
	MachineID       string   `json:"machineId"`
	UpgradeStatus   int      `json:"status"`
	UpgradeErrorMsg string   `json:"msg"`
	TimeStamp       int64    `json:"timestamp"`
	SourceUrl       []string `json:"sourceUrl"`
	Version         string   `json:"version"`

	PreBuild        string `json:"preBuild"`
	NextShowVersion string `json:"nextShowVersion"`
	PreBaseline     string `json:"preBaseline"`
	NextBaseline    string `json:"nextBaseline"`
}

func postUpgrade(data string) error {
	config := NewConfig(path.Join(system.VarLibDir, "config.json"))
	url := config.PlatformUrl + "/api/v1/update/status"
	postCacheFile := "/var/cache/lastore/postupgrade.cache"
	dataVer := "upmsg v1.0"
	var datas []string
	if len(data) != 0 {
		datas = append(datas, data)
	}
	readFile, err := os.OpenFile(postCacheFile, os.O_RDONLY, 0600)
	if err != nil {
		logger.Warning(err)
	} else {
		defer readFile.Close()
		reader := bufio.NewReader(readFile)
		line, _, err := reader.ReadLine() // 第一行为版本信息
		if err == nil && string(line) == dataVer {
			for {
				line, _, err = reader.ReadLine()
				if err == io.EOF {
					break
				}
				datas = append(datas, string(line))
				if len(datas) >= 100 { // 只处理100条
					break
				}
			}
		}
	}
	var retErr error
	var errDatas []string
	for _, data := range datas {
		func() {
			jsonstring, _ := base64.StdEncoding.DecodeString(data)
			logger.Debug(jsonstring)
			var postContent *upgradePostContent
			err := json.Unmarshal([]byte(jsonstring), &postContent)
			if err != nil {
				logger.Warning(err)
				errDatas = append(errDatas, data)
				return
			}
			postContent.TimeStamp = time.Now().Unix()
			content, err := json.Marshal(postContent)
			if err != nil {
				logger.Warning(err)
				errDatas = append(errDatas, data)
				return
			}
			logger.Debug(postContent)
			encryptMsg, err := updateplatform.EncryptMsg(content)
			if err != nil {
				logger.Warning(err)
				errDatas = append(errDatas, data)
				return
			}
			base64EncodeString := base64.StdEncoding.EncodeToString(encryptMsg)

			client := &http.Client{
				Timeout: 4 * time.Second,
			}
			request, err := http.NewRequest("POST", url, strings.NewReader(base64EncodeString))
			if err != nil {
				logger.Warning(err)
				retErr = err
				errDatas = append(errDatas, data)
			} else {
				response, err := client.Do(request)
				if err == nil {
					defer func() {
						_ = response.Body.Close()
					}()
					body, _ := io.ReadAll(response.Body)
					logger.Info(string(body))
				} else {
					logger.Warning(err)
					retErr = err
					errDatas = append(errDatas, data)
				}
			}
		}()
	}
	if len(errDatas) == 0 {
		err := os.RemoveAll(postCacheFile)
		if err != nil {
			retErr = err
		}
	} else {
		errDatas = append([]string{dataVer}, errDatas...)
		err := os.WriteFile(postCacheFile, []byte(strings.Join(errDatas, "\n")), 0600)
		if err != nil {
			retErr = err
		}
	}
	return retErr
}

func MainPostUpgrade(c *cli.Context) error {
	if c.NArg() == 1 {
		return postUpgrade(c.Args()[0])
	}
	return postUpgrade("")
}
