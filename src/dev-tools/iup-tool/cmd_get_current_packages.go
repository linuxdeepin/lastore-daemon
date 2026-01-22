package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var getCurrentPackagesCmd = &cobra.Command{
	Use:   "get_current_packages",
	Short: "Get current package lists from update platform",
	Long:  "Query the update platform for current version package lists",
	Run:   runGetCurrentPackages,
}

var currentPkgBaseline string

func init() {
	getCurrentPackagesCmd.Flags().StringVarP(&currentPkgBaseline, "baseline", "b", "", "Current baseline number (required)")
	_ = getCurrentPackagesCmd.MarkFlagRequired("baseline")
	rootCmd.AddCommand(getCurrentPackagesCmd)
}

func runGetCurrentPackages(cmd *cobra.Command, args []string) {
	if currentPkgBaseline == "" {
		logger.Warning("baseline is required")
		os.Exit(1)
	}

	response, err := genCurrentPkgListsResponse(updatePlatform.requestURL, updatePlatform.Token, currentPkgBaseline)
	if err != nil {
		logger.Warningf("genCurrentPkgListsResponse failed: %v", err)
		os.Exit(1)
	}

	data, err := getResponseData(response, GetCurrentPkgLists)
	if err != nil {
		logger.Warningf("getResponseData failed: %v", err)
		os.Exit(1)
	}

	pkgs := getCurrentPkgListsData(data)
	if pkgs == nil {
		logger.Warning("failed to parse current package list data")
		os.Exit(1)
	}

	logger.Infof("Current Package Lists for baseline: %s", currentPkgBaseline)
	logger.Infof("PreCheck scripts: %d", len(pkgs.PreCheck))
	logger.Infof("MidCheck scripts: %d", len(pkgs.MidCheck))
	logger.Infof("PostCheck scripts: %d", len(pkgs.PostCheck))
	logger.Infof("Core packages: %d", len(pkgs.Packages.Core))
	logger.Infof("Select packages: %d", len(pkgs.Packages.Select))
	logger.Infof("Freeze packages: %d", len(pkgs.Packages.Freeze))
	logger.Infof("Purge packages: %d", len(pkgs.Packages.Purge))
}

func genCurrentPkgListsResponse(requestUrl, token, baseline string) (*http.Response, error) {
	policyUrl := requestUrl + Urls[GetCurrentPkgLists].path
	client := &http.Client{
		Timeout: 40 * time.Second,
	}
	values := url.Values{}
	values.Add("baseline", baseline)
	policyUrl = policyUrl + "?" + values.Encode()
	request, err := http.NewRequest(Urls[GetCurrentPkgLists].method, policyUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", GetCurrentPkgLists.string(), err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(token)))
	logRequestHeaders(request)
	return client.Do(request)
}

func getCurrentPkgListsData(data json.RawMessage) *PreInstalledPkgMeta {
	tmp := &PreInstalledPkgMeta{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		logger.Warningf("%v failed to Unmarshal msg.Data to PreInstalledPkgMeta: %v ", GetCurrentPkgLists.string(), err.Error())
		return nil
	}
	return tmp
}
