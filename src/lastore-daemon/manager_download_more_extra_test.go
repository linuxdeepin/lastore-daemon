// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestCalculateTotalDownloadSizeNoMode(t *testing.T) {
	size, errs := calculateTotalDownloadSize(0, nil)
	assert.Equal(t, 0.0, size)
	assert.Empty(t, errs)
}

func TestCalculateTotalDownloadSizeEmptyPackages(t *testing.T) {
	// Exercises the QuerySourceDownloadSize branch with an empty package set.
	size, _ := calculateTotalDownloadSize(system.SystemUpdate, map[system.UpdateType][]string{})
	_ = size
}

func TestPrepareDistUpgradeImmutable(t *testing.T) {
	_, err := (&Manager{ImmutableAutoRecovery: true}).prepareDistUpgrade(ifcTestSender, system.SystemUpdate, initiatorUser)
	assert.Error(t, err)
}
