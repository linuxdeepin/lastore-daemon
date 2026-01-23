package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/cobra"
)

var getUpdateLogCmd = &cobra.Command{
	Use:   "get_update_log",
	Short: "Get system update logs from update platform",
	Long:  "Query the update platform for system update logs based on baseline and unstable version",
	Run:   runGetUpdateLog,
}

var (
	updateLogBaseline   string
	updateLogIsUnstable int
)

func init() {
	getUpdateLogCmd.Flags().StringVarP(&updateLogBaseline, "baseline", "b", "", "Target baseline number (required)")
	getUpdateLogCmd.Flags().IntVarP(&updateLogIsUnstable, "is-unstable", "u", 1, "Is unstable version (1 for release, 2 for unstable)")
	_ = getUpdateLogCmd.MarkFlagRequired("baseline")
	rootCmd.AddCommand(getUpdateLogCmd)
}

func runGetUpdateLog(cmd *cobra.Command, args []string) {
	if updateLogBaseline == "" {
		logger.Warning("baseline is required")
		os.Exit(1)
	}

	response, err := genUpdateLogResponse(updatePlatform.requestURL, updatePlatform.Token, updateLogBaseline, updateLogIsUnstable)
	if err != nil {
		logger.Warningf("genUpdateLogResponse failed: %v", err)
		os.Exit(1)
	}

	data, err := getResponseData(response, GetUpdateLog)
	if err != nil {
		logger.Warningf("getResponseData failed: %v", err)
		os.Exit(1)
	}

	logs := getUpdateLogData(data)
	if logs == nil {
		logger.Warning("failed to parse update log data")
		os.Exit(1)
	}
	logger.Debugf("Update Log Data: %v", spew.Sdump(logs))
}

func genUpdateLogResponse(requestUrl, token, baseline string, isUnstable int) (*http.Response, error) {
	policyUrl := requestUrl + Urls[GetUpdateLog].path
	client := newHTTPClient()
	values := url.Values{}
	values.Add("baseline", baseline)
	values.Add("isUnstable", fmt.Sprintf("%d", isUnstable))
	policyUrl = policyUrl + "?" + values.Encode()
	request, err := http.NewRequest(Urls[GetUpdateLog].method, policyUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", GetUpdateLog.string(), err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(token)))
	logRequestHeaders(request)
	return client.Do(request)
}

func getUpdateLogData(data json.RawMessage) []UpdateLogMeta {
	var tmp []UpdateLogMeta
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		logger.Warningf("%v failed to Unmarshal msg.Data to UpdateLogMeta: %v ", GetUpdateLog.string(), err.Error())
		return nil
	}
	return tmp
}
