/*
 * Copyright (C) 2017 ~ 2017 Deepin Technology Co., Ltd.
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

package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

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
		if err != nil {
			return err
		}
		if path == dirPath {
			return nil
		}
		if info.IsDir() {
			return filepath.SkipDir
		}
		// #nosec G304
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			_ = f.Close()
		}()
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
