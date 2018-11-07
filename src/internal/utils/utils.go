/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

func RunCommand(prog string, args ...string) (string, error) {
	buf := bytes.NewBuffer(nil)
	cmd := exec.Command(prog, args...)
	cmd.Stdout = (buf)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

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
	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	buf.ReadFrom(r)

	var lines []string
	var line string
	for ; err == nil; line, err = buf.ReadString('\n') {
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

func WriteData(fpath string, data interface{}) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}
	EnsureBaseDir(fpath)
	return ioutil.WriteFile(fpath, content, 0644)
}
