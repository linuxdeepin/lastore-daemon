// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubstr(t *testing.T) {
	tests := []struct {
		name   string
		str    string
		start  int
		length int
		want   string
	}{
		{"normal", "hello world", 0, 5, "hello"},
		{"from middle", "hello world", 6, 5, "world"},
		{"exceed length", "hello", 0, 10, "hello"},
		{"start beyond length", "hello", 10, 3, ""},
		{"negative start clamped", "hello", -1, 3, "hel"},
		{"negative length clamped", "hello", 0, -1, ""},
		{"empty string", "", 0, 5, ""},
		{"start at end", "hello", 5, 3, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Substr(tt.str, tt.start, tt.length)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPKCS7Encode(t *testing.T) {
	tests := []struct {
		name      string
		text      []byte
		blockSize int
		wantLen   int
	}{
		{"empty text", []byte{}, 32, 32},
		{"exact multiple", make([]byte, 32), 32, 64},
		{"less than block", []byte("hello"), 32, 32},
		{"one byte over", make([]byte, 33), 32, 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PKCS7Encode(tt.text, tt.blockSize)
			assert.Equal(t, tt.wantLen, len(result))
			padding := tt.blockSize - (len(tt.text) % tt.blockSize)
			if padding == 0 {
				padding = tt.blockSize
			}
			for i := len(tt.text); i < len(result); i++ {
				assert.Equal(t, byte(padding), result[i])
			}
		})
	}
}

func TestGetRandomBytes(t *testing.T) {
	t.Run("zero length", func(t *testing.T) {
		result, err := GetRandomBytes(0)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(result))
	})

	t.Run("non-zero length", func(t *testing.T) {
		result, err := GetRandomBytes(16)
		assert.NoError(t, err)
		assert.Equal(t, 16, len(result))
	})

	t.Run("different calls produce different results", func(t *testing.T) {
		r1, _ := GetRandomBytes(32)
		r2, _ := GetRandomBytes(32)
		assert.NotEqual(t, r1, r2)
	})
}

func TestEncryptMsg(t *testing.T) {
	t.Run("encrypt and verify length", func(t *testing.T) {
		data := []byte("test message for encryption")
		encrypted, err := EncryptMsg(data)
		assert.NoError(t, err)
		assert.NotNil(t, encrypted)
		// Encrypted length = (randomLen + len(data) + padding) which is multiple of BlockSize
		assert.Equal(t, 0, len(encrypted)%BlockSize)
		assert.True(t, len(encrypted) > len(data))
	})

	t.Run("encrypt empty data", func(t *testing.T) {
		data := []byte{}
		encrypted, err := EncryptMsg(data)
		assert.NoError(t, err)
		assert.NotNil(t, encrypted)
		assert.Equal(t, 0, len(encrypted)%BlockSize)
	})
}
