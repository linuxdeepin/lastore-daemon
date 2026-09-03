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

func TestDesensitize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "with user info",
			input: "Requested-By: alice (1000)",
			want:  "Requested-By: *** (***)",
		},
		{
			name:  "no user info",
			input: "some log line without user info",
			want:  "some log line without user info",
		},
		{
			name:  "multiple user info",
			input: "Requested-By: alice (1000)\nRequested-By: bob (1001)",
			want:  "Requested-By: *** (***)\nRequested-By: *** (***)",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, desensitize(tt.input))
		})
	}
}

func TestMaskLogfile(t *testing.T) {
	t.Run("nonexistent file", func(t *testing.T) {
		_, err := maskLogfile("/nonexistent/path/to/file.log")
		assert.Error(t, err)
	})

	t.Run("normal file copy", func(t *testing.T) {
		dir := t.TempDir()
		srcFile := filepath.Join(dir, "test.log")
		content := "line1\nline2\nline3\n"
		require.NoError(t, os.WriteFile(srcFile, []byte(content), 0644))

		outPath, err := maskLogfile(srcFile)
		require.NoError(t, err)
		assert.Equal(t, "/tmp/"+filepath.Base(srcFile), outPath)

		data, err := os.ReadFile(outPath)
		require.NoError(t, err)
		assert.Equal(t, content, string(data))
		_ = os.Remove(outPath)
	})

	t.Run("apt history log with desensitize", func(t *testing.T) {
		dir := t.TempDir()
		srcFile := filepath.Join(dir, "history.log")
		content := "Start-Date: 2024-01-01\nRequested-By: alice (1000)\nCommand: apt upgrade\n"
		require.NoError(t, os.WriteFile(srcFile, []byte(content), 0644))

		// maskLogfile only desensitizes when file == aptHistoryLog ("/var/log/apt/history.log")
		// For other files it just copies. So verify copy behavior.
		outPath, err := maskLogfile(srcFile)
		require.NoError(t, err)

		data, err := os.ReadFile(outPath)
		require.NoError(t, err)
		// Since the file path doesn't match aptHistoryLog, content should be copied as-is
		assert.Equal(t, content, string(data))
		_ = os.Remove(outPath)
	})
}
