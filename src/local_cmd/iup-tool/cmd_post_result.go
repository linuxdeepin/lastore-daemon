package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

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
	postResultCmd.Flags().StringVar(&resultDataFile, "data-file", "", "JSON file containing upgrade result data")
	postResultCmd.Flags().IntVar(&resultTaskID, "task-id", 0, "Task ID")
	postResultCmd.Flags().IntVar(&resultStatus, "status", 0, "Upgrade status (0:success, 1:failed, 2:check-failed)")
	postResultCmd.Flags().StringVar(&resultMsg, "message", "", "Error message (if failed)")
	postResultCmd.Flags().StringVar(&resultPreBaseline, "pre-baseline", "", "Previous baseline")
	postResultCmd.Flags().StringVar(&resultNextBaseline, "next-baseline", "", "Next baseline")
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
			TaskId:          resultTaskID,
			UpgradeStatus:   UpgradeResult(resultStatus),
			UpgradeErrorMsg: resultMsg,
			PreBaseline:     resultPreBaseline,
			NextBaseline:    resultNextBaseline,
			TimeStamp:       time.Now().Unix(),
		}
	}

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

func genPostResultResponse(requestUrl, token string, result *UpgradePostMsg) (*http.Response, error) {
	policyUrl := requestUrl + Urls[PostResult].path
	client := &http.Client{
		Timeout: 40 * time.Second,
	}

	resultData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result data: %v", err)
	}

	body := bytes.NewBuffer(resultData)
	request, err := http.NewRequest(Urls[PostResult].method, policyUrl, body)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", PostResult.string(), err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(token)))
	request.Header.Set("Content-Type", "application/json")
	return client.Do(request)
}
