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
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"

	"github.com/codegangsta/cli"
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
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(updateplatform.UpdateTokenConfigFile(c.IncludeDiskInfo, c.GetHardwareIdByHelper))))
	return client.Do(request)
}

// genVersionResponseFn and checkPolicyCacheFile are injectable seams so tests
// can exercise MainCheckPolicy without real network or shared state.
var (
	genVersionResponseFn = genVersionResponse
	// checkPolicyCacheFile 保存上次平台响应 md5 与时间的缓存。
	// 原路径为 /tmp/checkpolicy.cache：路径固定可预测且 /tmp 为任意用户可写，
	// 普通用户可抢先创建同名符号链接/命名管道，诱导 root 定时任务
	// （systemd-run --unit=lastoreCronCheck 执行 lastore-tools checkpolicy）读写其指定对象。
	checkPolicyCacheFile = CheckPolicyCacheFile
)

// readCheckPolicyCache 读取缓存的平台响应 md5 与时间；缓存缺失或不可信时返回零值。
func readCheckPolicyCache(cacheFile string) (string, time.Time) {
	var oldSum string
	oldTime := time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local)
	// O_NOFOLLOW 不跟随符号链接，避免缓存被替换为指向其它文件的链接；
	// O_NONBLOCK 保证即使缓存路径被替换为 FIFO 也不会阻塞读操作
	readFile, err := os.OpenFile(cacheFile, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		logger.Warning(err)
		return oldSum, oldTime
	}
	defer readFile.Close()
	if fi, err := readFile.Stat(); err != nil || !fi.Mode().IsRegular() {
		logger.Warningf("invalid check policy cache %s: %v", cacheFile, err)
		return oldSum, oldTime
	}
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
	return oldSum, oldTime
}

// writeCheckPolicyCache 以同目录临时文件 + rename 原子写入缓存，避免按路径截断写入。
func writeCheckPolicyCache(cacheFile, sum string, t time.Time) error {
	if err := utils.EnsureSecureDir(filepath.Dir(cacheFile)); err != nil {
		return err
	}
	content := sum + "\n" + t.Format(time.RFC3339) + "\n"
	return utils.WriteFileSecurely(cacheFile, []byte(content), 0600)
}

// MainCheckPolicy 检查更新策略，策略变化拉起lastore-daemon处理
func MainCheckPolicy(c *cli.Context) error {
	config := NewConfig(path.Join("/var/lib/lastore", "config.json"))
	cacheFile := checkPolicyCacheFile
	nowTime := time.Now()
	// 获取缓存的md5码
	oldSum, oldTime := readCheckPolicyCache(cacheFile)
	logger.Debug("Check old time:", oldTime)
	response, err := genVersionResponseFn(config)
	if err != nil {
		logger.Warning(err)
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	logger.Debug(string(body))
	if response.StatusCode == 200 {
		sum := md5.Sum(body)
		newSum := hex.EncodeToString(sum[:])
		logger.Debug("new md5 sum: ", newSum)
		if oldSum != newSum {
			if err := writeCheckPolicyCache(cacheFile, newSum, nowTime); err != nil {
				logger.Warning(err)
			}
			sysBus, err := systemBusFn()
			if err == nil {
				err = sysBus.Object("org.deepin.dde.Lastore1", "/org/deepin/dde/Lastore1").Call(
					"org.deepin.dde.Lastore1.Manager.UpdateSource", 0).Err
				if err != nil {
					logger.Warning(err)
				}

			}
		}
	}
	return nil
}
