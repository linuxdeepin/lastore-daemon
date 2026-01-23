package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

var getCVEInfoCmd = &cobra.Command{
	Use:   "get_cve_info",
	Short: "Get CVE vulnerability information from update platform",
	Long:  "Query the update platform for CVE (Common Vulnerabilities and Exposures) information",
	Run:   runGetCVEInfo,
}

var cveSyncTime string

func init() {
	getCVEInfoCmd.Flags().StringVarP(&cveSyncTime, "sync-time", "s", "", "Sync time for CVE data (optional, format: 2006-01-02)")
	rootCmd.AddCommand(getCVEInfoCmd)
}

func runGetCVEInfo(cmd *cobra.Command, args []string) {
	response, err := genCVEInfoResponse(updatePlatform.requestURL, updatePlatform.Token, cveSyncTime)
	if err != nil {
		logger.Warningf("genCVEInfoResponse failed: %v", err)
		os.Exit(1)
	}

	data, err := getResponseData(response, GetPkgCVEs)
	if err != nil {
		logger.Warningf("getResponseData failed: %v", err)
		os.Exit(1)
	}

	cveMeta := getCVEData(data)
	if cveMeta == nil {
		logger.Warning("failed to parse CVE data")
		os.Exit(1)
	}

	logger.Infof("CVE Data Time: %s", cveMeta.DataTime)
	logger.Infof("Total CVEs: %d", len(cveMeta.CVEs))
	logger.Infof("Total Packages with CVEs: %d", len(cveMeta.PkgCVEs))

	// 打印部分CVE信息
	count := 0
	for cveID, cveInfo := range cveMeta.CVEs {
		if count >= 10 {
			logger.Infof("... and %d more CVEs", len(cveMeta.CVEs)-10)
			break
		}
		logger.Infof("CVE %s: Severity=%s", cveID, cveInfo.Severity)
		logger.Debugf("  Description: %s", cveInfo.Description)
		count++
	}
}

func genCVEInfoResponse(requestUrl, token, syncTime string) (*http.Response, error) {
	policyUrl := requestUrl + Urls[GetPkgCVEs].path
	client := newHTTPClient()
	values := url.Values{}
	values.Add("synctime", syncTime)
	policyUrl = policyUrl + "?" + values.Encode()
	request, err := http.NewRequest(Urls[GetPkgCVEs].method, policyUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", GetPkgCVEs.string(), err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(token)))
	logRequestHeaders(request)
	return client.Do(request)
}

func getCVEData(data json.RawMessage) *CVEMeta {
	tmp := &CVEMeta{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		logger.Warningf("%v failed to Unmarshal msg.Data to CVEMeta: %v ", GetPkgCVEs.string(), err.Error())
		return nil
	}
	return tmp
}
