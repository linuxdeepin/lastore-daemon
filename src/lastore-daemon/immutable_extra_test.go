// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestOstreeParseRollbackData(t *testing.T) {
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	// The binary may or may not support this command; either way it should not panic
	data, err := mgr.osTreeParseRollbackData()
	if err != nil {
		assert.Empty(t, data)
	} else {
		// If successful, data should be valid JSON
		var raw json.RawMessage
		assert.NoError(t, json.Unmarshal([]byte(data), &raw))
	}
}

func TestOstreeCanRollback(t *testing.T) {
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	canRollback, dataJson := mgr.osTreeCanRollback()
	// Either returns false with empty string (error path) or true/false with data
	if dataJson == "" {
		assert.False(t, canRollback)
	}
}

func TestOstreeNeedRebootAfterRollback(t *testing.T) {
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	// Should not panic; returns false on error
	_ = mgr.osTreeNeedRebootAfterRollback()
}

func TestCheckFullMergeSupport(t *testing.T) {
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	// Should not panic; returns bool
	_ = mgr.checkFullMergeSupport()
}

func TestOsTreeRefreshError(t *testing.T) {
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	// osTreeRefresh calls osTreeCmd which may fail; should return error or nil
	_ = mgr.osTreeRefresh(false)
}

func TestOsTreeFinalizeError(t *testing.T) {
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	_ = mgr.osTreeFinalize()
}

func TestOsTreeRollbackError(t *testing.T) {
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	_ = mgr.osTreeRollback()
}
