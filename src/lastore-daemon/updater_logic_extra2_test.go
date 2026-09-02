// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

func TestGetUpdatablePackagesByType(t *testing.T) {
	u := &Updater{
		ClassifiedUpdatablePackages: map[string][]string{
			system.SystemUpdate.JobType():   {"pkg1", "pkg2"},
			system.SecurityUpdate.JobType(): {"pkg3"},
		},
	}

	result := u.getUpdatablePackagesByType(system.SystemUpdate)
	assert.ElementsMatch(t, []string{"pkg1", "pkg2"}, result)

	result = u.getUpdatablePackagesByType(system.SecurityUpdate)
	assert.ElementsMatch(t, []string{"pkg3"}, result)

	result = u.getUpdatablePackagesByType(system.SystemUpdate | system.SecurityUpdate)
	sort.Strings(result)
	assert.Equal(t, []string{"pkg1", "pkg2", "pkg3"}, result)
}

func TestGetUpdatablePackagesByType_Deduplication(t *testing.T) {
	u := &Updater{
		ClassifiedUpdatablePackages: map[string][]string{
			system.SystemUpdate.JobType():   {"pkg1", "pkg2"},
			system.SecurityUpdate.JobType(): {"pkg1", "pkg3"},
		},
	}

	result := u.getUpdatablePackagesByType(system.SystemUpdate | system.SecurityUpdate)
	sort.Strings(result)
	assert.Equal(t, []string{"pkg1", "pkg2", "pkg3"}, result)
}

func TestGetUpdatablePackagesByType_EmptyMap(t *testing.T) {
	u := &Updater{
		ClassifiedUpdatablePackages: map[string][]string{},
	}

	result := u.getUpdatablePackagesByType(system.SystemUpdate)
	assert.Empty(t, result)
}

func TestGetUpdatablePackagesByType_NoMatchingType(t *testing.T) {
	u := &Updater{
		ClassifiedUpdatablePackages: map[string][]string{
			system.SystemUpdate.JobType(): {"pkg1"},
		},
	}

	result := u.getUpdatablePackagesByType(system.SecurityUpdate)
	assert.Empty(t, result)
}

func TestGetUpdatablePackagesWithClassification(t *testing.T) {
	u := &Updater{
		ClassifiedUpdatablePackages: map[string][]string{
			system.SystemUpdate.JobType():   {"pkg1", "pkg2"},
			system.SecurityUpdate.JobType(): {"pkg3"},
		},
	}

	pkgs, pkgMap := u.getUpdatablePackagesWithClassification(system.SystemUpdate | system.SecurityUpdate)
	assert.Len(t, pkgs, 3)
	assert.Contains(t, pkgMap, system.SystemUpdate)
	assert.Contains(t, pkgMap, system.SecurityUpdate)
	assert.ElementsMatch(t, []string{"pkg1", "pkg2"}, pkgMap[system.SystemUpdate])
	assert.ElementsMatch(t, []string{"pkg3"}, pkgMap[system.SecurityUpdate])
}

func TestGetUpdatablePackagesWithClassification_Deduplication(t *testing.T) {
	u := &Updater{
		ClassifiedUpdatablePackages: map[string][]string{
			system.SystemUpdate.JobType():   {"pkg1", "pkg2"},
			system.SecurityUpdate.JobType(): {"pkg1", "pkg3"},
		},
	}

	pkgs, pkgMap := u.getUpdatablePackagesWithClassification(system.SystemUpdate | system.SecurityUpdate)
	assert.Len(t, pkgs, 3)
	assert.Len(t, pkgMap, 2)
}

func TestGetUpdatablePackagesWithClassification_EmptyMap(t *testing.T) {
	u := &Updater{
		ClassifiedUpdatablePackages: map[string][]string{},
	}

	pkgs, pkgMap := u.getUpdatablePackagesWithClassification(system.SystemUpdate)
	assert.Empty(t, pkgs)
	assert.Empty(t, pkgMap)
}

func TestWriteTimerFile_InvalidTimeFormat(t *testing.T) {
	changed, err := writeTimerFile("test", "invalid-time", "test.timer")
	assert.False(t, changed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse time")
}

func TestWriteTimerFile_EmptyUnit(t *testing.T) {
	changed, err := writeTimerFile("test", "10:30", "")
	assert.False(t, changed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unit is empty")
}

func TestNeedRefreshFullMerge_InvalidUid(t *testing.T) {
	result := needRefreshFullMerge(999999)
	assert.False(t, result)
}
