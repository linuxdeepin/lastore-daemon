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
	"os/exec"
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
