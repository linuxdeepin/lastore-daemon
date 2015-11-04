package main

import (
	"io/ioutil"
	"sort"
	"strings"
)

// TODO: write  tools to analyze the score of desktop in debs
// which has two or more desktop files.
// So we can know whether it is a reliable way to detect right desktop file.

type DesktopFiles []string

func (fs DesktopFiles) Len() int {
	return len(fs)
}
func (fs DesktopFiles) Swap(i, j int) {
	fs[i], fs[j] = fs[j], fs[i]
}
func (fs DesktopFiles) Less(i, j int) bool {
	return fs.score(i) < fs.score(j)
}

func (fs DesktopFiles) BestOne() string {
	if len(fs) == 0 {
		return ""
	}
	sort.Sort(fs)
	return fs[len(fs)-1]
}

func (fs DesktopFiles) score(i int) int {
	var score int
	bs, err := ioutil.ReadFile(fs[i])
	if err != nil {
		return -10
	}

	fpath := fs[i]
	content := string(bs)

	if !strings.Contains(content, "Exec=") {
		score = score - 10
	}
	if strings.Contains(content, "[Desktop Entry]") {
		score = score + 1
	} else {
		score = score - 10
	}

	if strings.Contains(content, "TryExec") {
		score = score + 5
	}
	if strings.Contains(content, "Type=Application") {
		score = score + 5
	}
	if strings.Contains(content, "StartupNotify") {
		score = score + 5
	}
	if strings.Contains(content, "Icon") {
		score = score + 3
	} else {
		score = score - 3
	}

	if strings.Contains(content, "NoDisplay=true") {
		score = score - 100
	}

	if strings.Contains(fpath, "/usr/share/applications") {
		score = score + 10
	}
	if strings.Contains(fpath, "/xsessions/") {
		score = score - 10
	}
	if strings.Contains(fs[i], "qtcreator/templates") {
		score = score - 5
	}
	if strings.Contains(fs[i], "autostart") {
		score = score - 1
	}
	if strings.Contains(fs[i], "desktop-base") {
		score = score - 5
	}
	if strings.Contains(fs[i], "xgreeters") {
		score = score - 5
	}

	return score
}
