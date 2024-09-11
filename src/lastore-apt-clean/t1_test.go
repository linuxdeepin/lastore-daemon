// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

	"github.com/stretchr/testify/assert"
)

func TestCompareVersion(t *testing.T) {
	tests := []struct {
		left, right string
		gt          bool
	}{
		{
			left:  "1.12+git+1+e37ca00-0.3",
			right: "1.12+git+1+e37ca0",
			gt:    true,
		},
		{
			left:  "1.12.2",
			right: "1.12.1",
			gt:    true,
		},
		{
			left:  "1.12.1",
			right: "1.12.1",
			gt:    false,
		},
		{
			left:  "1.12.1",
			right: "1.12.2",
			gt:    false,
		},
	}
	for _, test := range tests {
		gt := compareVersionsGt(test.left, test.right)
		assert.Equal(t, test.gt, gt)
	}
}

func Test_parseAptCachePolicyOutput(t *testing.T) {
	data := []byte(`bash:
  Installed: 5.0.1.1-1+dde
  Candidate: 5.0.1.1-1+dde
  Version table:
 *** 5.0.1.1-1+dde 100
        100 /usr/lib/dpkg-db/status
     5.0.1-1+deepin 500
        500 http://pools.example.com/dks-abc edf/main amd64 Packages
golang:
  Installed: (none)
  Candidate: 2:1.11~1
  Version table:
     2:1.11~1 500
        500 http://pools.example.com/dks-abc edf/main amd64 Packages
golang-dlib-dev:
  Installed: 5.6.0.11-1
  Candidate: 5.6.0.11-1
  Version table:
 *** 5.6.0.11-1 500
        500 http://pools.example.com/dks-abc edf/main amd64 Packages
        500 http://pools.example.com/dks-abc edf/main i386 Packages
        500 http://aptly.example.com/edf-1031/release unstable/main amd64 Packages
        500 http://aptly.example.com/edf-1031/release unstable/main i386 Packages
        100 /usr/lib/dpkg-db/status
     5.6.0.11-1 500
        500 http://ert.example.com/edf-1040/release-candidate/W0RERV3kuLvnur8xMDQw54mI5pys5YaF6YOo5rWL6K-VLTEyMjUyMDIwLTEyLTI1 unstable/main amd64 Packages
        500 http://ert.example.com/edf-1040/release-candidate/W0RERV3kuLvnur8xMDQw54mI5pys5YaF6YOo5rWL6K-VLTEyMjUyMDIwLTEyLTI1 unstable/main i386 Packages
`)
	reader := bytes.NewReader(data)
	result := parseAptCachePolicyOutput(reader)
	assert.Equal(t, "5.0.1.1-1+dde", result["bash"])
	assert.Equal(t, "2:1.11~1", result["golang"])
	assert.Equal(t, "5.6.0.11-1", result["golang-dlib-dev"])
}

func BenchmarkShouldDelete(b *testing.B) {
	findBins()

	dir, err := system.GetArchivesDir(system.LastoreAptV2ConfPath)
	if err != nil {
		b.Fatal(err)
	}
	fileInfos, err := os.ReadDir(dir)
	if err != nil {
		b.Fatal(err)
	}
	if len(fileInfos) == 0 {
		b.Skip("no file in dir")
	}

	cache, err := loadPkgStatusVersion()
	if err != nil {
		b.Fatal(err)
	}

	fileInfo := fileInfos[0]
	filename := filepath.Join(dir, fileInfo.Name())
	debInfo, err := getDebInfo(filename)
	if err != nil {
		b.Skip("get deb info failed")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		shouldDelete(debInfo, cache)
	}
}
