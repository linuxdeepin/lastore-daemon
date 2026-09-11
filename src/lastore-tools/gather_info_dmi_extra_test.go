// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMemorySizeByDmiSuccessExtra(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))
	script := `#!/bin/sh
cat <<'DMIDECODE'
Memory Device
	Size: 8 GB
Memory Device
	Size: 16 GB
DMIDECODE
exit 0
`
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "dmidecode"), []byte(script), 0755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	infos, err := getMemorySizeByDmi()
	require.NoError(t, err)
	require.Len(t, infos, 2)
	assert.Equal(t, "1", infos[0].MemoryNo)
	assert.Equal(t, int64(8*1024*1024*1024), infos[0].Capacity)
	assert.Equal(t, "2", infos[1].MemoryNo)
	assert.Equal(t, int64(16*1024*1024*1024), infos[1].Capacity)
}

func TestGetMemorySizeByDmiErrorExtra(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))
	script := "#!/bin/sh\necho 'permission denied' >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "dmidecode"), []byte(script), 0755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := getMemorySizeByDmi()
	require.Error(t, err)
}
