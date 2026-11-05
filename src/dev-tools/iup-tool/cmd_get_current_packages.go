package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

// CheckBaseDir is the daemon's deployment directory for platform check scripts.
const CheckBaseDir = "/var/lib/lastore/check"

var getCurrentPackagesCmd = &cobra.Command{
	Use:   "get_current_packages",
	Short: "Get current package lists from update platform",
	Long:  "Query the update platform for current version package lists",
	Run:   runGetCurrentPackages,
}

var currentPkgBaseline string
var currentPkgShowScripts bool

func init() {
	getCurrentPackagesCmd.Flags().StringVarP(&currentPkgBaseline, "baseline", "b", "", "Current baseline number (required)")
	_ = getCurrentPackagesCmd.MarkFlagRequired("baseline")
	getCurrentPackagesCmd.Flags().BoolVar(&currentPkgShowScripts, "show-scripts", false, "Print decoded check scripts without executing them")
	rootCmd.AddCommand(getCurrentPackagesCmd)
}

func runGetCurrentPackages(cmd *cobra.Command, args []string) {
	if currentPkgBaseline == "" {
		logger.Warning("baseline is required")
		osExit(1)
	}

	response, err := genCurrentPkgListsResponse(updatePlatform.requestURL, updatePlatform.Token, currentPkgBaseline)
	if err != nil {
		logger.Warningf("genCurrentPkgListsResponse failed: %v", err)
		osExit(1)
	}
	defer response.Body.Close()

	data, err := getResponseData(response, GetCurrentPkgLists)
	if err != nil {
		logger.Warningf("getResponseData failed: %v", err)
		osExit(1)
	}

	pkgs := getCurrentPkgListsData(data)
	if pkgs == nil {
		logger.Warning("failed to parse current package list data")
		osExit(1)
	}

	logger.Infof("Current Package Lists for baseline: %s", currentPkgBaseline)
	for _, group := range currentPackageScriptGroups(pkgs) {
		logger.Infof("%s scripts: %d", group.name, len(group.scripts))
	}
	if currentPkgShowScripts {
		if err := printCurrentPackageScripts(cmd.OutOrStdout(), pkgs); err != nil {
			logger.Warningf("show check scripts failed: %v", err)
			osExit(1)
		}
	}
	logger.Infof("Core packages: %d", len(pkgs.Packages.Core))
	logger.Infof("Select packages: %d", len(pkgs.Packages.Select))
	logger.Infof("Freeze packages: %d", len(pkgs.Packages.Freeze))
	logger.Infof("Purge packages: %d", len(pkgs.Packages.Purge))
}

// packageScriptGroup 关联平台字段名、daemon 部署子目录及该阶段的脚本。
type packageScriptGroup struct {
	name    string
	dir     string
	scripts []ShellCheck
}

func currentPackageScriptGroups(pkgs *PreInstalledPkgMeta) []packageScriptGroup {
	return []packageScriptGroup{
		// 平台保留历史字段 preCheck/midCheck/postCheck，daemon 则使用 *_upgrade_check
		// 目录；这里需与 PrepareCheckScripts 的映射一致，而非旧的 pre_check/mid_check/post_check。
		{"preCheck", "pre_upgrade_check", pkgs.PreCheck},
		{"midCheck", "mid_upgrade_check", pkgs.MidCheck},
		{"postCheck", "post_upgrade_check", pkgs.PostCheck},
		{"preUpdateCheck", "pre_update_check", pkgs.PreUpdateCheck},
		{"postUpdateCheck", "post_update_check", pkgs.PostUpdateCheck},
		{"preDownloadCheck", "pre_download_check", pkgs.PreDownloadCheck},
		{"postDownloadCheck", "post_download_check", pkgs.PostDownloadCheck},
		{"preBackupCheck", "pre_backup_check", pkgs.PreBackupCheck},
		{"postBackupCheck", "post_backup_check", pkgs.PostBackupCheck},
	}
}

func printCurrentPackageScripts(w io.Writer, pkgs *PreInstalledPkgMeta) error {
	for _, group := range currentPackageScriptGroups(pkgs) {
		for _, script := range group.scripts {
			// Match the daemon's handling of padded and unpadded Base64.
			content, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(script.Shell, "="))
			if err != nil {
				return fmt.Errorf("decode %s/%s: %w", group.name, terminalSafeText(script.Name, false), err)
			}
			destination := filepath.Join(CheckBaseDir, group.dir, script.Name)
			if _, err := fmt.Fprintf(w, "===== %s / %s =====\nLastore-daemon destination: %s\n\n%s\n", group.name, terminalSafeText(script.Name, false), terminalSafeText(destination, false), terminalSafeText(string(content), true)); err != nil {
				return err
			}
		}
	}
	return nil
}

// Render untrusted control and formatting characters visibly, not as terminal commands.
// Only script bodies retain line feeds and tabs; metadata must stay on one line.
func terminalSafeText(text string, multiline bool) string {
	var out strings.Builder
	for _, r := range text {
		if unicode.IsPrint(r) || (multiline && (r == '\n' || r == '\t')) {
			out.WriteRune(r)
		} else {
			quoted := strconv.QuoteRune(r)
			out.WriteString(quoted[1 : len(quoted)-1])
		}
	}
	return out.String()
}

func genCurrentPkgListsResponse(requestUrl, token, baseline string) (*http.Response, error) {
	policyUrl := requestUrl + Urls[GetCurrentPkgLists].path
	client := newHTTPClient()
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
