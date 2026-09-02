// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetControlField(t *testing.T) {
	tests := []struct {
		line    string
		key     string
		want    string
		wantErr bool
	}{
		{"Package: vim", "Package: ", "vim", false},
		{"Version: 2:8.1.1-1", "Version: ", "2:8.1.1-1", false},
		{"Architecture: amd64", "Architecture: ", "amd64", false},
		{"Wrong: field", "Package: ", "", true},
	}
	for _, tt := range tests {
		got, err := getControlField([]byte(tt.line), []byte(tt.key))
		if tt.wantErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		}
	}
}

func TestDebInfoPkgArch(t *testing.T) {
	di := &debInfo{pkg: "vim", version: "1.0", arch: "amd64"}
	assert.Equal(t, "vim:amd64", di.pkgArch())

	di2 := &debInfo{pkg: "bash", version: "5.0", arch: "all"}
	assert.Equal(t, "bash:all", di2.pkgArch())
}

func TestShouldDelete(t *testing.T) {
	tests := []struct {
		name        string
		debInfo     *debInfo
		cache       map[string]statusVersion
		wantPolicy  DeletePolicy
		wantTestAg  bool
	}{
		{
			name:       "not installed",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{},
			wantPolicy: DeleteExpired,
			wantTestAg: true,
		},
		{
			name:       "installed older deb",
			debInfo:    &debInfo{pkg: "vim", version: "2.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "ii", version: "1.0"}},
			wantPolicy: DeleteExpired,
			wantTestAg: true,
		},
		{
			name:       "installed same or older deb",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "ii", version: "2.0"}},
			wantPolicy: DeleteImmediately,
			wantTestAg: false,
		},
		{
			name:       "removed",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "rc", version: "1.0"}},
			wantPolicy: DeleteImmediately,
			wantTestAg: false,
		},
		{
			name:       "purged",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "pc", version: "1.0"}},
			wantPolicy: DeleteImmediately,
			wantTestAg: false,
		},
		{
			name:       "held",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "hi", version: "1.0"}},
			wantPolicy: DeleteImmediately,
			wantTestAg: false,
		},
		{
			name:       "unknown status",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "ui", version: "1.0"}},
			wantPolicy: DeleteExpired,
			wantTestAg: false,
		},
		{
			name:       "empty status",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "", version: "1.0"}},
			wantPolicy: DeleteExpired,
			wantTestAg: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, testAgain := shouldDelete(tt.debInfo, tt.cache)
			assert.Equal(t, tt.wantPolicy, policy)
			assert.Equal(t, tt.wantTestAg, testAgain)
		})
	}
}

func TestShouldDeleteTestAgain(t *testing.T) {
	// Save and restore candidate cache
	origCache := _candidateCache
	defer func() { _candidateCache = origCache }()

	_candidateCache = map[string]string{
		"vim:amd64": "1.0",
		"vim":       "1.0",
	}

	// Candidate version matches
	di := &debInfo{pkg: "vim", version: "1.0", arch: "amd64"}
	assert.Equal(t, DeletePolicy(Keep), shouldDeleteTestAgain(di))

	// Candidate version different
	di2 := &debInfo{pkg: "vim", version: "2.0", arch: "amd64"}
	assert.Equal(t, DeletePolicy(DeleteImmediately), shouldDeleteTestAgain(di2))

	// No candidate version
	di3 := &debInfo{pkg: "nonexistent", version: "1.0", arch: "amd64"}
	assert.Equal(t, DeletePolicy(DeleteExpired), shouldDeleteTestAgain(di3))

	// arch all uses pkg only
	_candidateCache = map[string]string{"vim": "1.0"}
	di4 := &debInfo{pkg: "vim", version: "1.0", arch: "all"}
	assert.Equal(t, DeletePolicy(Keep), shouldDeleteTestAgain(di4))
}

func TestParseAptCachePolicyOutput(t *testing.T) {
	input := `vim:
  Installed: 2:8.1.0875-1
  Candidate: 2:8.1.0875-1
bash:
  Installed: 5.0-4
  Candidate: 5.0-5
`
	result := parseAptCachePolicyOutput(strings.NewReader(input))
	assert.Equal(t, "2:8.1.0875-1", result["vim"])
	assert.Equal(t, "5.0-5", result["bash"])
}

func TestParseAptCachePolicyOutputEmpty(t *testing.T) {
	result := parseAptCachePolicyOutput(strings.NewReader(""))
	assert.Empty(t, result)
}

func TestCompareVersionsGt(t *testing.T) {
	assert.True(t, compareVersionsGt("2.0", "1.0"))
	assert.False(t, compareVersionsGt("1.0", "2.0"))
	assert.False(t, compareVersionsGt("1.0", "1.0"))
}
