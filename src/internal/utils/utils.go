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
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
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

// OpenURL open the url for reading
// It will reaturn error if open failed or the
// StatusCode is bigger than 299
// NOTE: the return reader need be closed
func OpenURL(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > 299 {
		resp.Body.Close()
		return nil, fmt.Errorf("OpenURL %q failed %q", url, resp.Status)
	}
	return resp.Body, nil
}

// TeeToFile invoke the handler with a new io.Reader which created by
// TeeReader in and the fpath's writer
func TeeToFile(in io.Reader, fpath string, handler func(io.Reader) error) error {
	_, err := os.Stat(path.Dir(fpath))
	if err != nil {
		err = os.MkdirAll(path.Dir(fpath), 0755)
		if err != nil {
			return err
		}
	}

	out, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer out.Close()

	tee := io.TeeReader(in, out)

	return handler(tee)
}
