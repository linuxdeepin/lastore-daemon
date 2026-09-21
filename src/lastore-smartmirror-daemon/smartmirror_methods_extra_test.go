// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/linuxdeepin/go-lib/dbusutil"
	nativesmartmirror "github.com/linuxdeepin/lastore-daemon/src/internal/smartmirror"
	"github.com/stretchr/testify/assert"
)

func TestRecordActualDownloadNeedsMethodTimeAndPersists(t *testing.T) {
	mirror := "https://fast.example/deepin/"
	s := &SmartMirror{
		sourcesURL:    []string{mirror},
		mirrorQuality: MirrorQuality{QualityMap: QualityMap{}, weights: map[string]int{}},
		qualityPath:   filepath.Join(t.TempDir(), "quality.json"),
		qualityDirty:  make(chan struct{}, 1),
	}
	uri := "https://fast.example/deepin/beige/pool/pkg.deb"

	// Without a method-side duration the sample must not produce a speed.
	if err := s.RecordActualDownload(uri, mirror, 1<<20, 0, 0, false, true); err != nil {
		t.Fatalf("record actual download: %v", err)
	}
	if speed := s.mirrorQuality.QualityMap[mirror].ActualSpeed; speed != 0 {
		t.Fatalf("speed recorded without method time: %v", speed)
	}

	if err := s.RecordActualDownload(uri, mirror, 1<<20, 1000, 0, false, true); err != nil {
		t.Fatalf("record actual download: %v", err)
	}
	s.flushQuality()

	data, err := os.ReadFile(s.qualityPath)
	if err != nil {
		t.Fatalf("read quality file: %v", err)
	}
	var saved QualityMap
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("decode quality file %s: %v", data, err)
	}
	if speed := saved[mirror].ActualSpeed; speed != float64(1<<20) {
		t.Fatalf("persisted speed = %v, file=%s", speed, data)
	}
}

func TestRecordActualDownloadFeedsRanking(t *testing.T) {
	now := time.Now()
	s := &SmartMirror{
		sourcesURL: []string{"https://slow.example/deepin/", "https://fast.example/deepin/"},
		mirrorQuality: MirrorQuality{
			QualityMap: QualityMap{},
			weights:    map[string]int{"https://slow.example/deepin/": 100, "https://fast.example/deepin/": 1},
		},
		qualityPath: filepath.Join(t.TempDir(), "quality.json"),
	}
	s.mirrorQuality.recordProbe2M("https://fast.example/deepin/", "https://fast.example/deepin/pkg.deb", nativesmartmirror.Sample{
		Bytes: probe2MBytes, Body: time.Second / 5, At: now,
	}, nil)
	if err := s.RecordActualDownload(
		"https://slow.example/deepin/beige/pool/pkg.deb",
		"https://slow.example/deepin/", 1<<20, 1000, 0, false, true,
	); err != nil {
		t.Fatalf("record actual download: %v", err)
	}
	ranked := s.mirrorQuality.rank(s.sourcesURL, time.Now())
	assert.Equal(t, "https://fast.example/deepin/", ranked[0])
}

func TestSmartMirrorSetEnableUnchanged(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "smartmirror_config.json")
	s := &SmartMirror{
		service: dbusutil.NewService(nil),
		config:  newConfig(tmp),
		Enable:  true,
	}

	err := s.SetEnable(true)
	assert.Nil(t, err)
	assert.True(t, s.Enable)
	assert.True(t, s.config.Enable)
}

func TestSmartMirrorQueryDisabled(t *testing.T) {
	s := &SmartMirror{service: dbusutil.NewService(nil), Enable: false}

	url, err := s.Query("https://official/pool/x.deb", "https://official", "https://mirror")
	assert.Nil(t, err)
	assert.Equal(t, "https://mirror/pool/x.deb", url)
}

func TestSmartMirrorGetExportedMethods(t *testing.T) {
	s := &SmartMirror{}
	methods := s.GetExportedMethods()
	assert.Len(t, methods, 3)

	names := []string{methods[0].Name, methods[1].Name, methods[2].Name}
	assert.ElementsMatch(t, []string{"Query", "SetEnable", "RecordActualDownload"}, names)
}

func TestActualMirrorMatchesURIAndBase(t *testing.T) {
	candidate := "https://mirror.example/deepin/"
	assert.True(t, actualMirrorMatches(candidate, "https://mirror.example/deepin/pool/main/a.deb"))
	assert.True(t, actualMirrorMatches(candidate, "https://mirror.example/deepin/"))
	assert.False(t, actualMirrorMatches(candidate, "https://other.example/deepin/pool/main/a.deb"))
}
