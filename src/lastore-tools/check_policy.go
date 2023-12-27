// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	. "internal/config"
	"internal/updateplatform"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/codegangsta/cli"
	"github.com/godbus/dbus"
)

var CMDCheckPolicy = cli.Command{
	Name:   "checkpolicy",
	Usage:  `check update policy`,
	Action: MainCheckPolicy,
}

func genVersionResponse(c *Config) (*http.Response, error) {
	url := c.PlatformUrl
	policyUrl := url + "/api/v1/version"
	client := &http.Client{
		Timeout: 4 * time.Second,
	}
	request, err := http.NewRequest("GET", policyUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", "/api/v1/version", err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(updateplatform.UpdateTokenConfigFile(c.IncludeDiskInfo))))
	return client.Do(request)
}

// MainCheckPolicy 检查更新策略，策略变化拉起lastore-daemon处理
func MainCheckPolicy(c *cli.Context) error {
	config := NewConfig(path.Join("/var/lib/lastore", "config.json"))
	cacheFile := "/tmp/checkpolicy.cache"
	var oldSum string
	oldTime := time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local)
	nowTime := time.Now()
	// 获取缓存的md5码
	readFile, err := os.OpenFile(cacheFile, os.O_RDONLY, 0666)
	if err != nil {
		logger.Warning(err)
	} else {
		defer readFile.Close()
		reader := bufio.NewReader(readFile)
		for {
			var str string
			data, _, err := reader.ReadLine()
			if err == io.EOF {
				break
			}
			str = string(data)
			oldSum = str
			data, _, err = reader.ReadLine()
			if err == io.EOF {
				break
			}
			str = string(data)
			oldTime, _ = time.Parse(time.RFC3339, str)
			break
		}
	}
	logger.Debug("Check old time:", oldTime)
	response, err := genVersionResponse(config)
	if err == nil {
		defer func() {
			_ = response.Body.Close()
		}()
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			logger.Warning(err)
			return nil
		}
		logger.Info(response.Body)
		if response.StatusCode == 200 {
			sum := md5.Sum(body)
			newSum := hex.EncodeToString(sum[:])
			if oldSum != newSum {
				writeFile, err := os.OpenFile(cacheFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
				if err == nil {
					defer writeFile.Close()
					writeFile.WriteString(newSum)
					writeFile.WriteString("\n")
					writeFile.WriteString(nowTime.Format(time.RFC3339))
					writeFile.WriteString("\n")
				}
				// oldSum为空时，只保存缓存(此时为刚启动，lastore-daemon会检查一次更新)，不为空时才拉起lastore-daemon
				if len(oldSum) != 0 {
					sysBus, err := dbus.SystemBus()
					if err == nil {
						_ = sysBus.Object("com.deepin.lastore", dbus.ObjectPath("/com/deepin/lastore")).Call(
							"com.deepin.lastore.Manager.UpdateSource", 0).Err
					}
				}
			}
		}
	} else {
		logger.Warning(err)
	}
	return nil
}
