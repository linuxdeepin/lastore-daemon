// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSmartMirrorQueryEnabled(t *testing.T) {
	s := &SmartMirror{service: dbusutil.NewService(nil), Enable: true}
	// route returns the raw URL when the input is not a valid URL.
	url, err := s.Query("not-a-url", "https://official", "https://mirror")
	assert.Nil(t, err)
	assert.Equal(t, "not-a-url", url)
}

func TestSmartMirrorQuerySuffixNormalized(t *testing.T) {
	s := &SmartMirror{service: dbusutil.NewService(nil), Enable: false}
	// mirrorHost without trailing slash is normalized before substitution.
	url, err := s.Query("https://official/pool/x.deb", "https://official", "https://mirror")
	assert.Nil(t, err)
	assert.Equal(t, "https://mirror/pool/x.deb", url)
}

func TestSmartMirrorRoutePool(t *testing.T) {
	s := &SmartMirror{}
	// No mirror sources configured -> makeChoice returns the original URL.
	original := "http://mirror.com/pool/main/x.deb"
	assert.Equal(t, original, s.route(original, "http://mirror.com"))
}

func TestSmartMirrorRouteByHash(t *testing.T) {
	s := &SmartMirror{}
	original := "http://mirror.com/dists/stable/main/binary-amd64/by-hash/SHA256/hash"
	assert.Equal(t, original, s.route(original, "http://mirror.com"))
}

func TestSmartMirrorMakeChoiceEmptySources(t *testing.T) {
	s := &SmartMirror{
		mirrorQuality: MirrorQuality{
			QualityMap:   QualityMap{},
			adjustDelays: map[string]int{},
		},
	}
	original := "http://mirror.com/pool/x.deb"
	assert.Equal(t, original, s.makeChoice(original, "http://mirror.com"))
}

func TestCompareFailedRatio(t *testing.T) {
	mq := &MirrorQuality{QualityMap: QualityMap{}, adjustDelays: map[string]int{}}
	mq.QualityMap["left"] = &Quality{AverageDelay: 100, AccessCount: 10, FailedCount: 5}
	mq.QualityMap["right"] = &Quality{AverageDelay: 100, AccessCount: 10, FailedCount: 1}
	assert.False(t, mq.compare("left", "right"))
	assert.True(t, mq.compare("right", "left"))
}

func TestCompareEqualRatioDelay(t *testing.T) {
	mq := &MirrorQuality{QualityMap: QualityMap{}, adjustDelays: map[string]int{}}
	mq.QualityMap["left"] = &Quality{AverageDelay: 50, AccessCount: 10, FailedCount: 1}
	mq.QualityMap["right"] = &Quality{AverageDelay: 200, AccessCount: 10, FailedCount: 1}
	assert.True(t, mq.compare("left", "right"))
	assert.False(t, mq.compare("right", "left"))
}

func TestCompareAdjustDelay(t *testing.T) {
	mq := &MirrorQuality{QualityMap: QualityMap{}, adjustDelays: map[string]int{"left": 1000}}
	mq.QualityMap["left"] = &Quality{AverageDelay: 50, AccessCount: 10, FailedCount: 0}
	mq.QualityMap["right"] = &Quality{AverageDelay: 200, AccessCount: 10, FailedCount: 0}
	// 50 + 1000 > 200 + 0 -> left is not better.
	assert.False(t, mq.compare("left", "right"))
}

func TestDetectSelectMirror(t *testing.T) {
	mq := &MirrorQuality{QualityMap: QualityMap{}, adjustDelays: map[string]int{}}
	mirrors := []string{"m1", "m2", "m3", "m4", "m5", "m6"}
	result := mq.detectSelectMirror(mirrors)
	assert.Len(t, result, 5)

	seen := map[string]bool{}
	for _, m := range result {
		assert.False(t, seen[m], "duplicate mirror %q", m)
		seen[m] = true
		assert.Equal(t, 1, mq.QualityMap[m].DetectCount)
	}
}

func TestUserAgentEmptyOutput(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "lsb_release"), []byte("#!/bin/sh\nexit 0\n"), 0755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ua := userAgent()
	assert.Contains(t, ua, "deepin unknown")
}
