// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package querydesktop

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

func ListAppStore(t *testing.T) []string { //nolint
	apps, err := utils.RunCommand("lastore-tools", "test", "-j", "search")
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(apps, "\n")
}

func ListInstalled(t *testing.T) []string { // nolint
	s, err := utils.RunCommand("bash", "-c", `dpkg -l | awk '{print $2}'`)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(s, "\n")
}

func Test_Len(t *testing.T) {
	dFile := DesktopFiles{
		PkgName: "",
		Files: []string{
			"0.desktop",
			"1.desktop",
		},
	}
	assert.Equal(t, 2, dFile.Len())
}

func Test_Swap(t *testing.T) {
	dFile := DesktopFiles{
		PkgName: "",
		Files: []string{
			"0.desktop",
			"1.desktop",
		},
	}
	dFile.Swap(0, 1)
	assert.Equal(t, "1.desktop", dFile.Files[0])
	assert.Equal(t, "0.desktop", dFile.Files[1])
}

func Test_score(t *testing.T) {
	dFile := DesktopFiles{
		PkgName: "testdata",
		Files: []string{
			"./testdata/electron-ssr.desktop",
			"./testdata/google-chrome.desktop",
			"./testdata/isomaster.desktop",
		},
	}
	assert.Equal(t, 29, dFile.score(0))
	assert.Equal(t, 34, dFile.score(1))
	assert.Equal(t, 34, dFile.score(2))
}

func Test_BestOne(t *testing.T) {
	dFile := DesktopFiles{
		PkgName: "testdata",
		Files: []string{
			"./testdata/electron-ssr.desktop",
			"./testdata/google-chrome.desktop",
		},
	}
	assert.Equal(t, "./testdata/google-chrome.desktop", dFile.BestOne())
}

func Test_Less(t *testing.T) {
	dFile := DesktopFiles{
		PkgName: "testdata",
		Files: []string{
			"./testdata/electron-ssr.desktop",
			"./testdata/google-chrome.desktop",
		},
	}
	assert.True(t, dFile.Less(0, 1))
}
