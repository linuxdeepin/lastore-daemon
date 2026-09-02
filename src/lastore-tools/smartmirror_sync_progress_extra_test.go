// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSaveMirrorInfos(t *testing.T) {
	infos := []MirrorInfo{
		{Name: "server1", Progress: 0.5, Support2014: true, Support2015: false},
		{Name: "server2", Progress: 1.0, Support2014: true, Support2015: true},
	}
	var buf bytes.Buffer
	err := SaveMirrorInfos(infos, &buf)
	assert.NoError(t, err)

	var decoded []MirrorInfo
	err = json.Unmarshal(buf.Bytes(), &decoded)
	assert.NoError(t, err)
	assert.Len(t, decoded, 2)
	assert.Equal(t, "server1", decoded[0].Name)
}

func TestSaveMirrorInfosEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := SaveMirrorInfos(nil, &buf)
	assert.NoError(t, err)
}

func TestU2014(t *testing.T) {
	assert.Equal(t, "http://example.com/dists/trusty/Release", u2014("http://example.com"))
	assert.Equal(t, "http://example.com/dists/trusty/Release", u2014("http://example.com/"))
}

func TestU2015(t *testing.T) {
	assert.Equal(t, "http://example.com/dists/unstable/Release", u2015("http://example.com"))
	assert.Equal(t, "http://example.com/dists/unstable/Release", u2015("http://example.com/"))
}

func TestUGuards(t *testing.T) {
	guards := []string{"g0", "g1", "g2", "g3", "g4", "g5", "g6", "g7", "g8", "g9"}
	result := uGuards("http://example.com", guards)
	assert.Len(t, result, 2)
	assert.Equal(t, "http://example.com/g0", result[0])
	assert.Equal(t, "http://example.com/g5", result[1])
}

func TestUGuardsEmpty(t *testing.T) {
	result := uGuards("http://example.com", nil)
	assert.Empty(t, result)
}
