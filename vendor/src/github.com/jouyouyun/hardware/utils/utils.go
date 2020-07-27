package utils

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"strings"
)

// ReadFileContent get file content and trim the last newline
func ReadFileContent(filename string) (string, error) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(contents), "\n"), nil
}

// SHA256Sum sum data by sha256
func SHA256Sum(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// ScanDir return needed sub directory
func ScanDir(dir string, filter func(string) bool) ([]string, error) {
	finfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var list []string
	for _, finfo := range finfos {
		if filter(finfo.Name()) {
			continue
		}
		list = append(list, finfo.Name())
	}
	return list, nil
}
