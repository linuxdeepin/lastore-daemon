// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package utils

/*
#include <stdlib.h>
*/
import "C"
import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/linuxdeepin/go-lib/keyfile"
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
		_ = cmd.Process.Kill()
	})
	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	_, _ = buf.ReadFrom(r)

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

// WriteData 把数据 data 序列化为 JSON 格式写入 fpath 路径的文件。
func WriteData(fpath string, data interface{}) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_ = EnsureBaseDir(fpath)
	return os.WriteFile(fpath, content, 0644)
}

func ValidURL(url string) bool {
	return strings.HasPrefix(url, "http")
}

// UnsetEnv 清除指定名称 envName 的环境变量
func UnsetEnv(envName string) (err error) {
	doUnsetEnvC(envName) // call C.unsetenv() is necessary
	envs := os.Environ()
	newEnvsData := make(map[string]string)
	for _, e := range envs {
		a := strings.SplitN(e, "=", 2)
		var name, value string
		if len(a) == 2 {
			name = a[0]
			value = a[1]
		} else {
			name = a[0]
			value = ""
		}
		if name != envName {
			newEnvsData[name] = value
		}
	}
	os.Clearenv()
	for e, v := range newEnvsData {
		err = os.Setenv(e, v)
		if err != nil {
			return
		}
	}
	return
}

func doUnsetEnvC(envName string) {
	cname := C.CString(envName)
	defer C.free(unsafe.Pointer(cname))
	C.unsetenv(cname)
}

// RuntimeDirPath 是 lastore 的 root 独占运行时目录：lastore-daemon.service 声明了
// RuntimeDirectory=lastore、RuntimeDirectoryMode=0700，普通用户无法在其中创建条目，
// 守护进程生成的临时文件/目录都应放在这里。
// 禁止使用 /tmp（1777 全局可写）：文件名一旦固定或可预测，本地用户即可抢先创建
// 同名符号链接/命名管道/普通文件，诱导 root 读写其指定对象（任意文件覆写、阻塞）。
var RuntimeDirPath = "/run/lastore"

// EnsureSecureDir 确保 dir 存在、是目录、不可被其它用户写入且属主为当前用户，
// 避免 root 把文件写进他人可预置条目的目录。
// 未由 systemd 创建 RuntimeDirectory 的一次性进程（如 lastore-tools）也依赖它建目录。
func EnsureSecureDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	if fi.Mode().Perm()&0022 != 0 {
		return fmt.Errorf("%s mode %#o is writable by other users", dir, fi.Mode().Perm())
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("%s is not owned by current user", dir)
	}
	return nil
}

// EnsureRuntimeDir 确保 RuntimeDirPath 可用
func EnsureRuntimeDir() error {
	return EnsureSecureDir(RuntimeDirPath)
}

func WriteFileSecurely(path string, content []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}

	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		_ = tmpFile.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = tmpFile.Write(content); err != nil {
		return err
	}
	if err = tmpFile.Chmod(perm); err != nil {
		return err
	}
	if err = tmpFile.Sync(); err != nil {
		return err
	}
	if err = tmpFile.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return err
	}

	cleanup = false
	return nil
}

func SaveKeyFileSecurely(path string, kf *keyfile.KeyFile, perm os.FileMode) error {
	var buf bytes.Buffer
	if err := kf.SaveToWriter(&buf); err != nil {
		return err
	}
	return WriteFileSecurely(path, buf.Bytes(), perm)
}
