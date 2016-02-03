/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
package main

import "dbus/com/deepin/lastore"
import log "github.com/cihub/seelog"
import "fmt"
import "net/http"
import "encoding/json"

func getApps() []string {

	resp, err := http.Get("http://api.appstore.deepin.org/info/all")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	apps := make(map[string]interface{})
	d := json.NewDecoder(resp.Body)
	err = d.Decode(&apps)
	if err != nil {
		panic(err)
	}

	var ids []string
	for app, _ := range apps {
		ids = append(ids, app)
	}
	fmt.Println("XXOO:", ids)
	return ids
}
func getLastore() *lastore.Manager {
	m, err := lastore.NewManager("com.deepin.lastore", "/com/deepin/lastore")
	if err != nil {
		panic(err)
	}
	return m
}

func RemoveAll() []string {
	m := getLastore()
	ids := getApps()
	var r []string
	for _, id := range ids {
		p, err := m.RemovePackage("Robot..."+id, id)
		if err != nil {
			log.Errorf("RemovePackage %q %v\n", id, err)
		}
		r = append(r, string(p))
	}
	return r
}

func InstallAll() []string {
	m := getLastore()
	ids := getApps()

	var r []string
	for _, id := range ids {
		p, err := m.InstallPackage("Roboot..."+id, id)
		if err != nil {
			log.Errorf("InstallPackage %q %v\n", id, err)
		}
		r = append(r, string(p))
	}
	return r
}
