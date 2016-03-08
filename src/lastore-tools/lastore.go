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
import "pkg.deepin.io/lib/dbus"
import "net/http"
import "encoding/json"
import "fmt"
import "time"
import "internal/system"
import "strings"

func getLastore() *lastore.Manager {
	m, err := lastore.NewManager("com.deepin.lastore", "/com/deepin/lastore")
	if err != nil {
		panic(err)
	}
	return m
}

func LastoreRemove(p string) error {
	m := getLastore()
	fmt.Println("Connected lastore-daemon..")

	fmt.Println("Try removing", p)
	j, err := m.RemovePackage("RemoveForTesing "+p, p)
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}

func LastoreInstall(p string) error {
	m := getLastore()
	fmt.Println("Connected lastore-daemon..")

	fmt.Println("Try installing", p)
	j, err := m.InstallPackage("InstallForTesing "+p, p)
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}

func LastoreSearch(server string, p string, debug bool) error {
	resp, err := http.Get(server + "/info/all")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	apps := make(map[string]interface{})
	d := json.NewDecoder(resp.Body)
	err = d.Decode(&apps)
	if err != nil {
		return err
	}

	for app, info := range apps {
		if p == "" || strings.Contains(app, p) {
			if debug {
				fmt.Printf("-%s %v\n", app, info)
			} else {
				fmt.Println(app)
			}
		}
	}
	return nil
}

func waitJob(p dbus.ObjectPath) error {
	j, err := lastore.NewJob("com.deepin.lastore", p)
	if err != nil {
		return err
	}

	s := j.Status.Get()
	for s != "" {
		fmt.Printf("id:%v(%v,%v) %q\n",
			j.Id.Get(), s, j.Progress.Get(),
			j.Description.Get())

		switch system.Status(s) {
		case system.ReadyStatus, system.RunningStatus:
		case system.PausedStatus:
			return fmt.Errorf("Job be paused.")
		case system.SucceedStatus:
			fmt.Println("Succeeful finished.")
			return nil
		case system.FailedStatus:
			return fmt.Errorf("Job %v failed %v", p, j)
		}

		time.Sleep(time.Millisecond * 300)
		s = j.Status.Get()
	}
	return err
}

func LastoreUpgrade() error {
	m := getLastore()
	fmt.Println("Connected lastore-daemon..")

	fmt.Println("Try updating /var/lib/apt/lists .....")
	j, err := m.UpdateSource()

	if err != nil {
		fmt.Printf("Created Job: %v failed\n", err)
		return err
	}
	if err = waitJob(j); err != nil {
		return err
	}
	fmt.Printf("Created Job: %q successful\n", j)

	fmt.Println()

	list := m.UpgradableApps.Get()
	if len(list) == 0 {
		fmt.Println("There hasn't any packages need be upgrade.")
		return nil
	}

	fmt.Printf("Try upgrading %v\n", list)
	j, err = m.DistUpgrade()
	if err != nil {
		return err
	}
	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}
