// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package cache

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckKey(t *testing.T) {
	assert.True(t, checkKey("Package"))
	assert.True(t, checkKey("Version"))
	assert.True(t, checkKey("Package-Name"))
	assert.True(t, checkKey("123"))
	assert.True(t, checkKey(""))
	assert.True(t, checkKey("Package Name"))  // space is in valid range
	assert.False(t, checkKey("Package@Name"))
	assert.False(t, checkKey("测试"))
}

func TestCheckValue(t *testing.T) {
	assert.True(t, checkValue("hello"))
	assert.True(t, checkValue("hello\nworld"))
	assert.True(t, checkValue("hello\tworld"))
	assert.True(t, checkValue(""))
	assert.False(t, checkValue("hello\x00world"))
	assert.False(t, checkValue("\x1F"))
	assert.True(t, checkValue("\x80"))
}

func TestEncode(t *testing.T) {
	var buf bytes.Buffer
	err := Encode(&buf, "Package", "vim")
	assert.NoError(t, err)
	assert.Equal(t, "Package:  vim\n", buf.String())
}

func TestEncodeMultiline(t *testing.T) {
	var buf bytes.Buffer
	err := Encode(&buf, "Description", "line1\nline2")
	assert.NoError(t, err)
	assert.Equal(t, "Description:  line1\n line2\n", buf.String())
}

func TestEncodeBadKey(t *testing.T) {
	var buf bytes.Buffer
	err := Encode(&buf, "Bad@Key", "value")
	assert.Error(t, err)
}

func TestEncodeBadValue(t *testing.T) {
	var buf bytes.Buffer
	err := Encode(&buf, "Package", "bad\x00value")
	assert.Error(t, err)
}

func TestEncodeMap(t *testing.T) {
	var buf bytes.Buffer
	stanza := Stanza{
		"Package": "vim",
		"Version": "1.0",
	}
	err := EncodeMap(&buf, stanza)
	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "vim")
	assert.Contains(t, output, "1.0")
}

func TestEncodeMapSkipEmpty(t *testing.T) {
	var buf bytes.Buffer
	stanza := Stanza{
		"Package": "",
		"Version": "1.0",
	}
	err := EncodeMap(&buf, stanza)
	assert.NoError(t, err)
	output := buf.String()
	assert.NotContains(t, output, "Package")
	assert.Contains(t, output, "1.0")
}

func TestEncodeMapBadKey(t *testing.T) {
	var buf bytes.Buffer
	stanza := Stanza{
		"Bad@Key": "value",
	}
	err := EncodeMap(&buf, stanza)
	assert.Error(t, err)
}

func TestDecode(t *testing.T) {
	input := "Package: vim\nVersion: 1.0\n\n"
	stanza, err := Decode(strings.NewReader(input))
	assert.NoError(t, err)
	assert.Equal(t, "vim", stanza["Package"])
	assert.Equal(t, "1.0", stanza["Version"])
}

func TestDecodeMultiline(t *testing.T) {
	input := "Description: line1\n line2\n\n"
	stanza, err := Decode(strings.NewReader(input))
	assert.NoError(t, err)
	assert.Equal(t, "line1\nline2", stanza["Description"])
}

func TestDecodeEmpty(t *testing.T) {
	stanza, err := Decode(strings.NewReader(""))
	assert.NoError(t, err)
	assert.Empty(t, stanza)
}

func TestDecodeMalformed(t *testing.T) {
	input := "no-colon-line\n\n"
	_, err := Decode(strings.NewReader(input))
	assert.Error(t, err)
}

func TestSoftwareString(t *testing.T) {
	sw := Software{
		Package: "vim",
		Version: "1.0",
		Architecture: "amd64",
	}
	s := sw.String()
	assert.Contains(t, s, "vim")
	assert.Contains(t, s, "1.0")
	assert.Contains(t, s, "amd64")
}

func TestSoftwareEncode(t *testing.T) {
	sw := Software{
		Package: "vim",
		Version: "1.0",
		Architecture: "amd64",
	}
	bts, err := sw.Encode()
	assert.NoError(t, err)
	output := string(bts)
	assert.Contains(t, output, "Package:  vim")
	assert.Contains(t, output, "Version:  1.0")
	assert.Contains(t, output, "Architecture:  amd64")
}

func TestSoftwareEncodeEmpty(t *testing.T) {
	sw := Software{}
	bts, err := sw.Encode()
	assert.NoError(t, err)
	assert.Empty(t, bts)
}

func TestSoftwareStanza(t *testing.T) {
	sw := &Software{}
	sw.Stanza(map[string]string{
		"Package": "vim",
		"Version": "1.0",
		"Architecture": "amd64",
	})
	assert.Equal(t, "vim", sw.Package)
	assert.Equal(t, "1.0", sw.Version)
	assert.Equal(t, "amd64", sw.Architecture)
}

func TestSoftwareDependsList(t *testing.T) {
	tests := []struct {
		name    string
		depends string
		want    [][2]string
		wantErr bool
	}{
		{
			name:    "empty",
			depends: "",
			want:    nil,
		},
		{
			name:    "single",
			depends: "libc6 (>= 2.31)",
			want:    [][2]string{{"libc6", ">= 2.31"}},
		},
		{
			name:    "multiple",
			depends: "libc6 (>= 2.31), libvim (>= 8.0)",
			want: [][2]string{
				{"libc6", ">= 2.31"},
				{"libvim", ">= 8.0"},
			},
		},
		{
			name:    "missing parens",
			depends: "libc6",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sw := &Software{Depends: tt.depends}
			got, err := sw.DependsList()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestMergePackagesSoftware(t *testing.T) {
	sw1 := &Software{Package: "vim", Version: "1.0", Architecture: "amd64", SHA256: "abc"}
	sw2 := &Software{Package: "vim", Version: "1.0", Architecture: "amd64", SHA256: "abc"}
	result, err := MergePackagesSoftware(false, []*Software{sw1}, []*Software{sw2})
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestMergePackagesSoftwareStrictMismatch(t *testing.T) {
	sw1 := &Software{Package: "vim", Version: "1.0", Architecture: "amd64", SHA256: "abc"}
	sw2 := &Software{Package: "vim", Version: "1.0", Architecture: "amd64", SHA256: "def"}
	_, err := MergePackagesSoftware(true, []*Software{sw1}, []*Software{sw2})
	assert.Error(t, err)
}

func TestMergePackagesSoftwareWarm(t *testing.T) {
	sw1 := &Software{Package: "vim", Version: "1.0", Architecture: "amd64"}
	sw2 := &Software{Package: "vim", Version: "1.0", Architecture: "amd64"}
	result, same, err := MergePackagesSoftwareWarm([]*Software{sw1}, []*Software{sw2})
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Len(t, same, 1)
}

func TestMergePackagesSoftwareEmpty(t *testing.T) {
	result, err := MergePackagesSoftware(false)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestMakeStanzaAppInfo(t *testing.T) {
	stanza := Stanza{
		"Package":       "vim",
		"Version":       "1.0",
		"Architecture":  "amd64",
		"Filename":      "pool/main/v/vim/vim.deb",
		"SHA256":        "abc123",
		"Size":          "1024",
		"Installed-Size": "2048",
	}
	ok, info := makeStanzaAppInfo(stanza)
	assert.True(t, ok)
	assert.Equal(t, "vim", info.Name)
	assert.Equal(t, "1.0", info.Version)
	assert.Equal(t, "amd64", info.Arch)
	assert.Equal(t, "vim.deb", info.Filename)
	assert.Equal(t, "abc123", info.HashSha256)
	assert.Equal(t, 1024, info.DebSize)
	assert.Equal(t, 2048, info.InstalledSize)
}

func TestMakeStanzaAppInfoMissingFields(t *testing.T) {
	stanza := Stanza{"Package": "vim"}
	ok, _ := makeStanzaAppInfo(stanza)
	assert.False(t, ok)
}

func TestMakeStanzaAppInfoInvalidSize(t *testing.T) {
	stanza := Stanza{
		"Package":       "vim",
		"Version":       "1.0",
		"Architecture":  "amd64",
		"Filename":      "vim.deb",
		"SHA256":        "abc",
		"Size":          "not-a-number",
		"Installed-Size": "also-not",
	}
	ok, info := makeStanzaAppInfo(stanza)
	assert.True(t, ok)
	assert.Equal(t, 0, info.DebSize)     // invalid size stays at zero value
	assert.Equal(t, 0, info.InstalledSize) // invalid installed-size stays at zero value
}

func TestDecodeStanza(t *testing.T) {
	input := "Package: vim\nVersion: 1.0\nArchitecture: amd64\n\n"
	transform := func(s map[string]string) (interface{}, error) {
		return s["Package"], nil
	}
	result, err := DecodeStanza(strings.NewReader(input), transform)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "vim", result[0])
}

func TestDecodeStanzaEmpty(t *testing.T) {
	result, err := DecodeStanza(strings.NewReader(""), nil)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestDecodeStanzaByList(t *testing.T) {
	input := "Package: vim\nVersion: 1.0\n\nPackage: bash\nVersion: 5.0\n\n"
	transform := func(s map[string]string) (interface{}, error) {
		return s["Package"], nil
	}
	result, err := DecodeStanzaByList(strings.NewReader(input), transform, []string{"vim"})
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "vim", result[0])
}

func TestDecodeStanzaByListEmpty(t *testing.T) {
	result, err := DecodeStanzaByList(strings.NewReader(""), nil, []string{"vim"})
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestDecodeStanzaByCacheInfo(t *testing.T) {
	result, err := DecodeStanzaByCacheInfo(strings.NewReader(""), nil, &CacheInfo{})
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestIsMultilineField(t *testing.T) {
	assert.True(t, isMultilineField("", false))
	assert.True(t, isMultilineField("Description", false))
	assert.True(t, isMultilineField("Files", false))
	assert.True(t, isMultilineField("Changes", false))
	assert.True(t, isMultilineField("Checksums-Sha1", false))
	assert.True(t, isMultilineField("Checksums-Sha256", false))
	assert.True(t, isMultilineField("Checksums-Sha512", false))
	assert.True(t, isMultilineField("Package-List", false))
	assert.False(t, isMultilineField("MD5Sum", false))
	assert.True(t, isMultilineField("MD5Sum", true))
	assert.False(t, isMultilineField("SHA1", false))
	assert.True(t, isMultilineField("SHA1", true))
	assert.True(t, isMultilineField("SHA256", true))
	assert.True(t, isMultilineField("SHA512", true))
	assert.False(t, isMultilineField("Package", false))
}

func TestCanonicalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"package", "Package"},
		{"version", "Version"},
		{"installed-size", "Installed-Size"},
		{"SHA1", "SHA1"},
		{"SHA256", "SHA256"},
		{"SHA512", "SHA512"},
		{"md5sum", "MD5Sum"},
		{"MD5SUM", "MD5Sum"},
		{"notautomatic", "NotAutomatic"},
		{"butautomaticupgrades", "ButAutomaticUpgrades"},
		{"sha256", "SHA256"},
		{"package-name", "Package-Name"},
	}
	for _, tt := range tests {
		got := canonicalCase(tt.input)
		assert.Equal(t, tt.want, got, "input=%q", tt.input)
	}
}

func TestNewControlFileReader(t *testing.T) {
	r := strings.NewReader("Package: vim\n\n")
	cfr := NewControlFileReader(r, false, false)
	assert.NotNil(t, cfr)
}

func TestReadStanza(t *testing.T) {
	input := "Package: vim\nVersion: 1.0\nArchitecture: amd64\n\n"
	cfr := NewControlFileReader(strings.NewReader(input), false, false)
	stanza, err := cfr.ReadStanza()
	require.NoError(t, err)
	assert.Equal(t, "vim", stanza["Package"])
	assert.Equal(t, "1.0", stanza["Version"])
	assert.Equal(t, "amd64", stanza["Architecture"])
}

func TestReadStanzaMultiple(t *testing.T) {
	input := "Package: vim\nVersion: 1.0\n\nPackage: bash\nVersion: 5.0\n\n"
	cfr := NewControlFileReader(strings.NewReader(input), false, false)
	stanza1, err := cfr.ReadStanza()
	require.NoError(t, err)
	assert.Equal(t, "vim", stanza1["Package"])
	stanza2, err := cfr.ReadStanza()
	require.NoError(t, err)
	assert.Equal(t, "bash", stanza2["Package"])
	stanza3, err := cfr.ReadStanza()
	require.NoError(t, err)
	assert.Nil(t, stanza3)
}

func TestReadStanzaMalformed(t *testing.T) {
	input := "no colon here\n"
	cfr := NewControlFileReader(strings.NewReader(input), false, false)
	_, err := cfr.ReadStanza()
	assert.Error(t, err)
}

func TestReadStanzaMultilineField(t *testing.T) {
	input := "Description: short summary\n long description line 1\n\n"
	cfr := NewControlFileReader(strings.NewReader(input), false, false)
	stanza, err := cfr.ReadStanza()
	require.NoError(t, err)
	assert.Contains(t, stanza["Description"], "short summary")
	// continuation lines starting with space are skipped in non-installer mode
}

func TestReadStanzaEmpty(t *testing.T) {
	cfr := NewControlFileReader(strings.NewReader(""), false, false)
	stanza, err := cfr.ReadStanza()
	assert.NoError(t, err)
	assert.Nil(t, stanza)
}

func TestReadStanzaInstallerMode(t *testing.T) {
	input := "Package: vim\nVersion: 1.0\n ignored line\n\n"
	cfr := NewControlFileReader(strings.NewReader(input), false, true)
	// in installer mode all non-empty lines are skipped
	stanza, err := cfr.ReadStanza()
	assert.NoError(t, err)
	assert.Nil(t, stanza)
}

func TestDiffPackagesSoftwareOverlap(t *testing.T) {
	sw1 := &Software{Package: "vim", Version: "1.0", Architecture: "amd64"}
	sw2 := &Software{Package: "bash", Version: "5.0", Architecture: "amd64"}
	sw3 := &Software{Package: "vim", Version: "1.0", Architecture: "amd64"}
	sw4 := &Software{Package: "git", Version: "2.0", Architecture: "amd64"}
	ownl, ownr := DiffPackagesSoftware([]*Software{sw1, sw2}, []*Software{sw3, sw4})
	assert.Len(t, ownl, 1)
	assert.Equal(t, "bash", ownl[0].Package)
	assert.Len(t, ownr, 1)
	assert.Equal(t, "git", ownr[0].Package)
}

func TestSoftwareStringError(t *testing.T) {
	sw := Software{Package: "bad\x01value"}
	s := sw.String()
	assert.Equal(t, "错误的Value值", s)
}

func TestMakeStanzaAppInfoMissingEachField(t *testing.T) {
	base := Stanza{
		"Package":       "vim",
		"Version":       "1.0",
		"Architecture":  "amd64",
		"Filename":      "pool/main/v/vim/vim.deb",
		"SHA256":        "abc123",
		"Size":          "1024",
		"Installed-Size": "2048",
	}
	for _, field := range []string{"Package", "Version", "Architecture", "Filename", "SHA256"} {
		t.Run("missing-"+field, func(t *testing.T) {
			s := Stanza{}
			for k, v := range base {
				s[k] = v
			}
			delete(s, field)
			ok, _ := makeStanzaAppInfo(s)
			assert.False(t, ok, "missing %s should return false", field)
		})
	}
}

func TestMakeStanzaAppInfoNoSizeFields(t *testing.T) {
	s := Stanza{
		"Package":      "vim",
		"Version":      "1.0",
		"Architecture": "amd64",
		"Filename":     "pool/main/v/vim/vim.deb",
		"SHA256":       "abc123",
	}
	ok, info := makeStanzaAppInfo(s)
	assert.True(t, ok)
	assert.Equal(t, -1, info.DebSize)
	assert.Equal(t, -1, info.InstalledSize)
}

func TestMakeStanzaAppInfoInvalidSizeNegative(t *testing.T) {
	s := Stanza{
		"Package":       "vim",
		"Version":       "1.0",
		"Architecture":  "amd64",
		"Filename":      "vim.deb",
		"SHA256":        "abc",
		"Size":          "-5",
		"Installed-Size": "-9",
	}
	ok, info := makeStanzaAppInfo(s)
	assert.True(t, ok)
	assert.Equal(t, -5, info.DebSize)
	assert.Equal(t, -9, info.InstalledSize)
}
