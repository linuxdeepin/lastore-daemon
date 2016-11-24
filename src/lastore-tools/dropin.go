package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

var CacheDir = "/var/lib/lastore/cache"
var BaseDir = "/lastore"

func BuildDesktop2uaid() (map[string]string, error) {
	// "dpath -> uaid"
	return buildMapStringStringInfo(filepath.Join(BaseDir, "override", "desktop2uaid"))
}

func BuildCategories() (map[string]string, error) {
	// "xdg category -> lastore category"
	return buildMapStringStringInfo(filepath.Join(BaseDir, "override", "xcategories"))
}

func handleDropinDir(dirPath string, handle func(f io.Reader) error) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if path == dirPath {
			return nil
		}
		if info.IsDir() {
			return filepath.SkipDir
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return handle(f)
	})
}

func buildMapStringStringInfo(dir string) (map[string]string, error) {
	var all = make(map[string]string)
	err := handleDropinDir(dir, func(f io.Reader) error {
		var t map[string]string
		d := json.NewDecoder(f)
		err := d.Decode(&t)
		for k, v := range t {
			if v == "" {
				delete(all, k)
			} else if k == "" {
				continue
			} else {
				all[k] = v
			}
		}
		return err
	})
	return all, err
}
