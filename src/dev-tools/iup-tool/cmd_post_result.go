package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/linuxdeepin/go-lib/utils"
	"github.com/spf13/cobra"
)

var postResultCmd = &cobra.Command{
	Use:   "post_result",
	Short: "Post upgrade result to update platform",
	Long:  "Send final upgrade result to the update platform for reporting",
	Run:   runPostResult,
}

var (
	resultDataFile     string
	resultTaskID       int
	resultStatus       int
	resultMsg          string
	resultPreBaseline  string
	resultNextBaseline string
)

func init() {
	postResultCmd.Flags().StringVarP(&resultDataFile, "data-file", "f", "", "JSON file containing upgrade result data")
	postResultCmd.Flags().IntVarP(&resultTaskID, "task-id", "T", 0, "Task ID")
	postResultCmd.Flags().IntVarP(&resultStatus, "status", "s", 0, "Upgrade status (0:success, 1:failed, 2:check-failed)")
	postResultCmd.Flags().StringVarP(&resultMsg, "message", "m", "", "Error message (if failed)")
	postResultCmd.Flags().StringVarP(&resultPreBaseline, "current-baseline", "c", "", "current Previous baseline")
	postResultCmd.Flags().StringVarP(&resultNextBaseline, "target-baseline", "t", "", "target Next baseline")
	rootCmd.AddCommand(postResultCmd)
}

func runPostResult(cmd *cobra.Command, args []string) {
	var result *UpgradePostMsg

	// 从文件读取或从命令行参数构建
	if resultDataFile != "" {
		data, err := os.ReadFile(resultDataFile)
		if err != nil {
			logger.Warningf("failed to read data file: %v", err)
			os.Exit(1)
		}
		result = &UpgradePostMsg{}
		err = json.Unmarshal(data, result)
		if err != nil {
			logger.Warningf("failed to parse JSON data: %v", err)
			os.Exit(1)
		}
	} else {
		result = &UpgradePostMsg{
			SerialNumber:     "", // TODO
			TaskId:           resultTaskID,
			MachineID:        updatePlatform.machineID,
			UpgradeStatus:    UpgradeResult(resultStatus),
			UpgradeErrorMsg:  resultMsg,
			PreBuild:         "", // TODO
			PreBaseline:      resultPreBaseline,
			NextBaseline:     resultNextBaseline,
			TimeStamp:        time.Now().Unix(),
			Version:          "",         // TODO
			NextShowVersion:  "",         // TODO
			SourceUrl:        []string{}, // TODO
			PostStatus:       "",         // TODO
			UpgradeEndTime:   time.Now().Unix(),
			UpgradeStartTime: time.Now().Add(-time.Minute).Unix(),
			Uuid:             utils.GenUuid(),
		}
	}
	logger.Debugf("Result: %s", spew.Sdump(result))

	response, err := genPostResultResponse(updatePlatform.requestURL, updatePlatform.Token, result)
	if err != nil {
		logger.Warningf("genPostResultResponse failed: %v", err)
		os.Exit(1)
	}

	data, err := getResponseData(response, PostResult)
	if err != nil {
		logger.Warningf("getResponseData failed: %v", err)
		os.Exit(1)
	}

	logger.Infof("Upgrade result posted successfully")
	logger.Debugf("Response data: %s", string(data))
}

// 相关函数 UpdatePlatformManager.PostSystemUpgradeMessage
func genPostResultResponse(requestURL, token string, result *UpgradePostMsg) (*http.Response, error) {
	policyURL := requestURL + Urls[PostResult].path
	client := newHTTPClient()

	resultData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result data: %v", err)
	}

	logger.Debugf("upgrade post content is %v", string(resultData))

	// Encrypt message using AES-CBC encryption
	encryptMsg, err := EncryptMsg(resultData)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt message: %v", err)
	}

	// Encode encrypted message using base64 standard encoding
	base64EncodeString := base64.StdEncoding.EncodeToString(encryptMsg)

	request, err := http.NewRequest(Urls[PostResult].method, policyURL, strings.NewReader(base64EncodeString))
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", PostResult.string(), err.Error())
	}

	logRequestHeaders(request)
	return client.Do(request)
}
