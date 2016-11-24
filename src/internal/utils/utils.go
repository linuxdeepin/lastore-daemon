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
	"bufio"
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
	return lines, err
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

// EnsureBaseDir make sure the parent directory of fpath exists
func EnsureBaseDir(fpath string) error {
	baseDir := path.Dir(fpath)
	info, err := os.Stat(baseDir)
	if err == nil && info.IsDir() {
		return nil
	}
	return os.MkdirAll(baseDir, 0755)
}

// TeeToFile invoke the handler with a new io.Reader which created by
// TeeReader in and the fpath's writer
func TeeToFile(in io.Reader, fpath string, handler func(io.Reader) error) error {
	if err := EnsureBaseDir(fpath); err != nil {
		return err
	}

	out, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer out.Close()

	tee := io.TeeReader(in, out)

	return handler(tee)
}

func RemoteCatLine(url string) (string, error) {
	in, err := OpenURL(url)
	if err != nil {
		return "", err
	}
	defer in.Close()

	r := bufio.NewReader(in)

	_line, isPrefix, err := r.ReadLine()
	line := string(_line)
	if isPrefix {
		return line, fmt.Errorf("the line %q is too long", line)
	}
	return line, err
}
