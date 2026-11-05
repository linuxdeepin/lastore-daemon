// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentPackageScriptsAllStages(t *testing.T) {
	fields := map[string]string{
		"preCheck": "pre_upgrade_check", "midCheck": "mid_upgrade_check", "postCheck": "post_upgrade_check",
		"preUpdateCheck": "pre_update_check", "postUpdateCheck": "post_update_check",
		"preDownloadCheck": "pre_download_check", "postDownloadCheck": "post_download_check",
		"preBackupCheck": "pre_backup_check", "postBackupCheck": "post_backup_check",
	}
	for field, dir := range fields {
		for name, encoding := range map[string]*base64.Encoding{"padded": base64.StdEncoding, "raw": base64.RawStdEncoding} {
			t.Run(field+"/"+name, func(t *testing.T) {
				content := "#!/bin/sh\necho inspect-only\n"
				data := fmt.Sprintf(`{%q:[{"name":"test.sh","shell":%q}]}`, field, encoding.EncodeToString([]byte(content)))
				pkgs := getCurrentPkgListsData([]byte(data))
				require.NotNil(t, pkgs)
				var out bytes.Buffer
				require.NoError(t, printCurrentPackageScripts(&out, pkgs))
				assert.Equal(t, "===== "+field+" / test.sh =====\nLastore-daemon destination: "+CheckBaseDir+"/"+dir+"/test.sh\n\n"+content+"\n", out.String())
			})
		}
	}
}

func TestCurrentPackageScriptsInvalidBase64(t *testing.T) {
	pkgs := &PreInstalledPkgMeta{PreCheck: []ShellCheck{{Name: "bad.sh", Shell: "!invalid!"}}}
	var out bytes.Buffer
	err := printCurrentPackageScripts(&out, pkgs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "preCheck/bad.sh")
}

func TestCurrentPackageScriptsEmpty(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, printCurrentPackageScripts(&out, &PreInstalledPkgMeta{}))
	assert.Empty(t, out.String())
}

func TestCurrentPackageScriptsTerminalControls(t *testing.T) {
	name := "bad\x1b[2J\n\r\t.sh"
	content := "#!/bin/sh\n\techo 中文\n\x1b]52;c;clipboard\a\r\b\x00\x7f\u009b\u202e"
	pkgs := &PreInstalledPkgMeta{PreCheck: []ShellCheck{{Name: name, Shell: base64.StdEncoding.EncodeToString([]byte(content))}}}
	var out bytes.Buffer
	require.NoError(t, printCurrentPackageScripts(&out, pkgs))
	safeName := `bad\x1b[2J\n\r\t.sh`
	assert.Contains(t, out.String(), "===== preCheck / "+safeName+" =====\n")
	assert.Contains(t, out.String(), "destination: "+CheckBaseDir+"/pre_upgrade_check/"+safeName+"\n")
	assert.Contains(t, out.String(), "#!/bin/sh\n\techo 中文\n"+`\x1b]52;c;clipboard\a\r\b\x00\x7f\u009b\u202e`)
	for _, control := range []string{"\x1b", "\a", "\r", "\b", "\x00", "\x7f", "\u009b", "\u202e"} {
		assert.NotContains(t, out.String(), control)
	}
	pkgs.PreCheck[0].Shell = "!invalid!"
	err := printCurrentPackageScripts(io.Discard, pkgs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), safeName)
	assert.NotContains(t, err.Error(), "\x1b")
}

func TestRunGetCurrentPackagesBaselineRequired(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	currentPkgBaseline = ""

	assert.Panics(t, func() { runGetCurrentPackages(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetCurrentPackagesResponseError(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	currentPkgBaseline = "b1"

	assert.Panics(t, func() { runGetCurrentPackages(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetCurrentPackagesDataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":false,"code":500,"msg":"server error"}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	currentPkgBaseline = "b1"

	assert.Panics(t, func() { runGetCurrentPackages(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetCurrentPackagesParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":42}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	currentPkgBaseline = "b1"

	assert.Panics(t, func() { runGetCurrentPackages(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetCurrentPackagesSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/package", r.URL.Path)
		assert.Equal(t, "b1", r.URL.Query().Get("baseline"))
		assert.NotEmpty(t, r.Header.Get("X-Repo-Token"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
			"result": true,
			"code": 0,
			"data": {
				"preCheck":  [{"name":"pre1","shell":"echo pre"}],
				"midCheck":  [{"name":"mid1","shell":"echo mid"}],
				"postCheck": [{"name":"post1","shell":"echo post"}],
				"packages": {
					"core":   [{"name":"core1","need":true,"allArchVersion":[{"arch":"amd64","version":"1.0"}]}],
					"select": [{"name":"sel1","need":false}],
					"freeze": [{"name":"frz1","need":false}],
					"purge":  [{"name":"prg1","need":false}]
				}
			}
		}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	currentPkgBaseline = "b1"

	runGetCurrentPackages(nil, nil)

	assert.Equal(t, 0, *code, "success path must not call osExit")
}
