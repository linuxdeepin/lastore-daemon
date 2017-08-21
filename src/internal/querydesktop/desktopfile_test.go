/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package querydesktop

import (
	"internal/utils"
	"strings"
	"testing"
)

func init() {
	InitDB()
}

func TestDesktopQuery(t *testing.T) {
	t.Skip("Appstore is broken nown")
	infos := ListInstalled(t)
	isInstalled := func(name string) bool {
		for _, i := range infos {
			if name == i {
				return true
			}
		}
		return false
	}
	for _, name := range ListAppStore(t) {
		if !isInstalled(name) {
			continue
		}
		result := QueryDesktopFile(name)
		if result == "" {
			t.Errorf("Query Failed at %q\n", name)
		}
	}
}

func ListAppStore(t *testing.T) []string {
	apps, err := utils.RunCommand("lastore-tools", "test", "-j", "search")
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(apps, "\n")
}

func ListInstalled(t *testing.T) []string {
	s, err := utils.RunCommand("bash", "-c", `dpkg -l | awk '{print $2}'`)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(s, "\n")
}
