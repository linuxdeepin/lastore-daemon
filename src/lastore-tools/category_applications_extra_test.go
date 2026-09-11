// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/dstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateApplicationsExported(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "applications.json")

	if _, err := dstore.NewStore().GetMetadataServer(); err != nil {
		t.Skipf("metadata server not configured: %v", err)
	}

	// Seed a fresh cache so dstore reads locally without hitting the network.
	require.NoError(t, os.WriteFile(fpath+".cache.json", []byte("{}"), 0644))

	assert.NoError(t, GenerateApplications("desktop", fpath))
}
