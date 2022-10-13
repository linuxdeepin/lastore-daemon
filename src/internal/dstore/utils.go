// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dstore

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Check file in cache
func cacheFetchJSON(v interface{}, url, cacheFilepath string, expire time.Duration) error {
	decodeFile := func() error {
		// #nosec G304
		f, err := os.Open(cacheFilepath)
		if err != nil {
			return err
		}

		jsonDec := json.NewDecoder(f)
		return jsonDec.Decode(v)
	}

	fi, _ := os.Stat(cacheFilepath)
	if (fi != nil) && (time.Since(fi.ModTime()) < expire) {
		return decodeFile()
	}

	client := http.DefaultClient
	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("User-Agent", "lastore-tools")
	request.Header.Add("Accept-Encoding", "gzip")
	if fi != nil {
		request.Header.Add("If-Modified-Since", fi.ModTime().Format(time.RFC1123))
	}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()
	lastModified, _ := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	if (fi != nil) && lastModified.Sub(fi.ModTime()) <= 0 {
		// update modify time
		now := time.Now()
		_ = os.Chtimes(cacheFilepath, now, now)
		return decodeFile()
	}

	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer func() {
			_ = reader.Close()
		}()
	default:
		reader = resp.Body
	}

	jsonDec := json.NewDecoder(reader)
	err = jsonDec.Decode(v)
	if err != nil {
		return err
	}

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	// #nosec G302 G304
	f, err := os.OpenFile(cacheFilepath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "failed to open cache file:", err)
		return nil
	}
	defer func() {
		_ = f.Close()
	}()
	_, err = f.Write(data)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "failed to write data to cache file:", err)
	}
	return nil
}
