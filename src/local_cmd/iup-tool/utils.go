package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/godbus/dbus/v5"
	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
)

// UpdatePlatformManager 更新平台管理器
type UpdatePlatformManager struct {
	requestURL string
	Token      string
}

const (
	dSettingsAppID          = "org.deepin.dde.lastore"
	dSettingsLastoreName    = "org.deepin.dde.lastore"
	dSettingsKeyPlatformUrl = "platform-url"
)

// getTokenFromAptConfig 从 apt-config 获取 Token
func getTokenFromAptConfig() string {
	cmd := exec.Command("apt-config", "dump", "Acquire::SmartMirrors::Token")
	output, err := cmd.Output()
	if err != nil {
		logger.Warningf("failed to get token from apt-config: %v", err)
		return ""
	}

	// 解析输出: Acquire::SmartMirrors::Token "token_value";
	line := strings.TrimSpace(string(output))
	if line == "" {
		logger.Warning("apt-config returned empty output")
		return ""
	}

	// 查找双引号之间的内容
	startIdx := strings.Index(line, "\"")
	if startIdx == -1 {
		logger.Warningf("failed to parse token: no opening quote found in: %s", line)
		return ""
	}
	endIdx := strings.LastIndex(line, "\"")
	if endIdx == -1 || endIdx <= startIdx {
		logger.Warningf("failed to parse token: no closing quote found in: %s", line)
		return ""
	}

	token := line[startIdx+1 : endIdx]
	logger.Debugf("Token loaded from apt-config: %s", token)
	return token
}

// getPlatformURLFromDSettings 从 dSettings 获取平台 URL
func getPlatformURLFromDSettings() string {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warningf("failed to get system bus: %v", err)
		return ""
	}

	ds := ConfigManager.NewConfigManager(sysBus)
	dsPath, err := ds.AcquireManager(0, dSettingsAppID, dSettingsLastoreName, "")
	if err != nil {
		logger.Warningf("failed to acquire dSettings manager: %v", err)
		return ""
	}

	dsManager, err := ConfigManager.NewManager(sysBus, dsPath)
	if err != nil {
		logger.Warningf("failed to create dSettings manager: %v", err)
		return ""
	}

	v, err := dsManager.Value(0, dSettingsKeyPlatformUrl)
	if err != nil {
		logger.Warningf("failed to get platform URL from dSettings: %v", err)
		return ""
	}

	url := v.Value().(string)
	logger.Debugf("Platform URL loaded from dSettings: %s", url)
	return url
}

// getClientPackageInfo 获取客户端包信息
func getClientPackageInfo(clientPackageName string) string {
	_ = clientPackageName
	return "client=lastore-daemon&version=6.2.45"
}

// getResponseData 解析 HTTP 响应数据
func getResponseData(response *http.Response, reqType requestType) (json.RawMessage, error) {
	if http.StatusOK == response.StatusCode {
		respData, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("%v failed to read response body: %v ", response.Request.RequestURI, err.Error())
		}
		logger.Debugf("%v request for %v respData:%s ", reqType.string(), response.Request.URL, string(respData))
		msg := &tokenMessage{}
		err = json.Unmarshal(respData, msg)
		if err != nil {
			logger.Warningf("%v request for %v respData:%s ", reqType.string(), response.Request.URL, string(respData))
			return nil, fmt.Errorf("%v failed to Unmarshal respData to tokenMessage: %v ", reqType.string(), err.Error())
		}
		if !msg.Result {
			logger.Warningf("%v request for %v respData:%s ", reqType.string(), response.Request.URL, string(respData))
			errorMsg := &tokenErrorMessage{}
			err = json.Unmarshal(respData, errorMsg)
			if err != nil {
				return nil, fmt.Errorf("%v request for %s", reqType.string(), response.Request.RequestURI)
			}
			return nil, fmt.Errorf("%v request for %s err:%s", reqType.string(), response.Request.RequestURI, errorMsg.Msg)
		}
		return msg.Data, nil
	} else {
		return nil, fmt.Errorf("request for %s failed, response code=%d", response.Request.RequestURI, response.StatusCode)
	}
}
