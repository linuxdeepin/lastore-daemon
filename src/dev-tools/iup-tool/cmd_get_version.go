package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/cobra"
)

// getVersionData 解析版本数据
func getVersionData(data json.RawMessage) *updateMessage {
	tmp := &updateMessage{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		logger.Warningf("%v failed to Unmarshal msg.Data to updateMessage: %v ", GetVersion.string(), err.Error())
		return nil
	}
	return tmp
}

// genVersionResponse 生成版本请求
func (m *UpdatePlatformManager) genVersionResponse() (*http.Response, error) {
	policyURL := m.requestURL + Urls[GetVersion].path
	client := newHTTPClient()
	request, err := http.NewRequest(Urls[GetVersion].method, policyURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", GetVersion.string(), err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(m.Token)))
	request.Header.Set("X-Packages", base64.RawStdEncoding.EncodeToString([]byte(getClientPackageInfo(""))))
	logRequestHeaders(request)
	return client.Do(request)
}

// genUpdatePolicyByToken 检查更新时将token数据发送给更新平台，获取本次更新信息
func (m *UpdatePlatformManager) genUpdatePolicyByToken() error {
	response, err := m.genVersionResponse()
	if err != nil {
		return fmt.Errorf("failed get version data %v", err)
	}
	logger.Debugf("response: %v", response)
	data, err := getResponseData(response, GetVersion)
	if err != nil {
		return fmt.Errorf("failed get version data %v", err)
	}
	msg := getVersionData(data)
	logger.Infof("msg: %s", spew.Sdump(msg))
	return nil
}

var getVersionCmd = &cobra.Command{
	Use:   "get_version",
	Short: "Get version information from update platform",
	Long:  "Query the update platform for version information and update policy",
	Run: func(cmd *cobra.Command, args []string) {
		err := updatePlatform.genUpdatePolicyByToken()
		if err != nil {
			logger.Warningf("genUpdatePolicyByToken failed: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(getVersionCmd)
}
