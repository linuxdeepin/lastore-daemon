/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
package main

import (
	"encoding/json"
	"fmt"
	"internal/system"
	"net/http"
)

func GenerateMirrors(repository string, fpath string) error {
	ms, err := LoadMirrorSources(fmt.Sprintf("http://api.lastore.deepin.org/mirrors?repository=%s", repository))
	if err != nil {
		return err
	}
	return writeData(fpath, ms)
}

// LoadMirrorSources return supported MirrorSource from remote server
func LoadMirrorSources(url string) ([]system.MirrorSource, error) {
	rep, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer rep.Body.Close()

	d := json.NewDecoder(rep.Body)
	var v struct {
		StatusCode    int    `json:"status_code"`
		StatusMessage string `json:"status_message"`
		Data          []struct {
			Id       string                       `json:"id"`
			Weight   int                          `json:"weight"`
			Name     string                       `json:"name"`
			Url      string                       `json:"url"`
			Location string                       `json:"location"`
			Locale   map[string]map[string]string `json:"locale"`
		} `json:"data"`
	}
	err = d.Decode(&v)
	if err != nil {
		return nil, err
	}

	if v.StatusCode != 0 {
		return nil, fmt.Errorf("LoadMirrorSources: featch(%q) error: %q",
			url, v.StatusMessage)
	}

	var r []system.MirrorSource
	for _, raw := range v.Data {
		s := system.MirrorSource{
			Id:         raw.Id,
			Name:       raw.Name,
			Url:        raw.Url,
			Weight:     raw.Weight,
			NameLocale: make(map[string]string),
		}
		for k, v := range raw.Locale {
			s.NameLocale[k] = v["name"]
		}
		r = append(r, s)
	}
	if len(r) == 0 {
		return nil, system.NotFoundError
	}
	return r, nil
}
