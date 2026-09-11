// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryDpkgUpgradeInfoByAptList(t *testing.T) {
	// A nonexistent source file makes ListDistUpgradePackages fail on os.Stat,
	// avoiding any apt execution.
	ps, err := queryDpkgUpgradeInfoByAptList(filepath.Join(t.TempDir(), "nonexistent.list"))
	assert.Error(t, err)
	assert.Nil(t, ps)
}

func TestGenerateUpdateInfos(t *testing.T) {
	err := GenerateUpdateInfos(filepath.Join(t.TempDir(), "update_infos.json"))
	t.Logf("GenerateUpdateInfos returned: %v", err)
}

func TestGenerateUpdateInfosWithSeam(t *testing.T) {
	old := queryDpkgUpgradeInfoByAptListFn
	t.Cleanup(func() { queryDpkgUpgradeInfoByAptListFn = old })

	calls := 0
	queryDpkgUpgradeInfoByAptListFn = func(sourcePath string) ([]string, error) {
		calls++
		switch calls {
		case 1:
			// Success: maps into upgradeInfo. "all" is always in the arch regex.
			return []string{"pkg/stable 1.2.3 all [upgradable from: 1.2.2]"}, nil
		case 2:
			// Covers the os.IsNotExist branch (source type absent).
			return nil, os.ErrNotExist
		default:
			// Covers the generic-error branch writing error_update_infos.json.
			return nil, errors.New("boom")
		}
	}

	outputPath := filepath.Join(t.TempDir(), "update_infos.json")
	require.NoError(t, GenerateUpdateInfos(outputPath))

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "pkg")
}
