/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
package utils

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func FilterExecOutput(cmd *exec.Cmd, timeout time.Duration, filter func(line string) bool) ([]string, error) {
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	errBuf := new(bytes.Buffer)
	cmd.Stderr = errBuf
	timer := time.AfterFunc(timeout, func() {
		cmd.Process.Kill()
	})
	cmd.Start()

	buf := bytes.NewBuffer(nil)
	buf.ReadFrom(r)

	var lines []string
	var line string
	for ; err == nil; line, err = buf.ReadString('\n') {
		errBuf.WriteString(line)
		line = strings.TrimSpace(line)
		if filter(line) {
			lines = append(lines, line)
		}
	}

	err = cmd.Wait()
	timer.Stop()
	if err != nil && len(lines) == 0 {
		return nil, fmt.Errorf("Run cmd %v --> %q(stderr) --> %v\n",
			cmd.Args, errBuf.String(), err)
	}
	return lines, nil
}

func ReportChoosedServer(official string, filename string, choosedServer string) error {
	official = AppendSuffix(official, "/")
	choosedServer = AppendSuffix(choosedServer, "/")
	const DetectVersion = "detector/0.1.1 " + runtime.GOARCH
	var userAgent string
	r, _ := exec.Command("lsb_release", "-ds").Output()
	if len(r) == 0 {
		userAgent = DetectVersion + " " + "deepin unknown"
	} else {
		userAgent = DetectVersion + " " + strings.TrimSpace(string(r))
	}
	bs, _ := ioutil.ReadFile("/etc/machine-id")

	req, err := http.NewRequest("HEAD", official+filename, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("MID", strings.TrimSpace(string(bs)))
	req.Header.Set("Mirror", choosedServer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		d, err := httputil.DumpResponse(resp, true)
		return fmt.Errorf("%s %s", string(d), err)
	}
	return nil
}

// AppendSuffix will append suffix to r and return
// the result string if the r hasn't the suffix before.
func AppendSuffix(r string, suffix string) string {
	if strings.HasSuffix(r, suffix) {
		return r
	}
	return r + suffix
}
