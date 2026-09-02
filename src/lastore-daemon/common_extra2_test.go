// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageJSONRoundTrip(t *testing.T) {
	pkg := Package{PkgName: "testpkg", Version: "1.2.3"}
	data, err := json.Marshal(pkg)
	require.NoError(t, err)

	var decoded Package
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, pkg, decoded)
}

func TestPackageListJSONRoundTrip(t *testing.T) {
	pl := PackageList{
		PkgList: []Package{
			{PkgName: "a", Version: "1"},
			{PkgName: "b", Version: "2"},
		},
		Version: "2.0",
	}
	data, err := json.Marshal(pl)
	require.NoError(t, err)

	var decoded PackageList
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, pl, decoded)
}

func TestPackageListJSONFieldNames(t *testing.T) {
	data := []byte(`{"PkgList":[{"PkgName":"x","Version":"3"}],"Version":"3.0"}`)
	var pl PackageList
	require.NoError(t, json.Unmarshal(data, &pl))
	assert.Len(t, pl.PkgList, 1)
	assert.Equal(t, "x", pl.PkgList[0].PkgName)
	assert.Equal(t, "3", pl.PkgList[0].Version)
	assert.Equal(t, "3.0", pl.Version)
}
