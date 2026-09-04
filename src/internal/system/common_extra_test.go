// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEditionName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "os-version")
	require.NoError(t, os.WriteFile(path, []byte("[Version]\nEditionName=Community\n"), 0644))

	edition, err := getEditionNameFromFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "Community", edition)
}
