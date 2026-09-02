// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package mirrors

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestMirrorsLen(t *testing.T) {
	m := mirrors{{Id: "a"}, {Id: "b"}, {Id: "c"}}
	assert.Equal(t, 3, m.Len())
	assert.Equal(t, 0, mirrors{}.Len())
}

func TestMirrorsLess(t *testing.T) {
	m := mirrors{{Weight: 10}, {Weight: 5}, {Weight: 20}}
	assert.True(t, m.Less(0, 1))
	assert.False(t, m.Less(1, 0))
	assert.True(t, m.Less(2, 1))
}

func TestMirrorsSwap(t *testing.T) {
	m := mirrors{{Id: "a", Weight: 1}, {Id: "b", Weight: 2}}
	m.Swap(0, 1)
	assert.Equal(t, "b", m[0].Id)
	assert.Equal(t, "a", m[1].Id)
}

func TestToMirrorsSourceList(t *testing.T) {
	m := mirrors{
		{Id: "m1", Name: "Mirror1", Weight: 10, UrlHttps: "mirror1.com", Country: "CN"},
		{Id: "m2", Name: "Mirror2", Weight: 5, UrlHttp: "mirror2.com", Country: "US"},
		{Id: "m3", Name: "Mirror3", Weight: 20, Country: "JP"},
	}
	result := toMirrorsSourceList(m)
	assert.Len(t, result, 2)
	assert.Equal(t, "m1", result[0].Id)
	assert.Equal(t, "https://mirror1.com", result[0].Url)
	assert.Equal(t, "m2", result[1].Id)
	assert.Equal(t, "http://mirror2.com", result[1].Url)
}

func TestToMirrorsSourceListWithLocale(t *testing.T) {
	m := mirrors{
		{
			Id:      "m1",
			Name:    "Mirror1",
			Weight:  10,
			UrlHttp: "m1.com",
			Locale: map[string]map[string]string{
				"zh_CN": {"name": "镜像1"},
				"en_US": {"name": "Mirror1"},
			},
		},
	}
	result := toMirrorsSourceList(m)
	assert.Len(t, result, 1)
	assert.Equal(t, "镜像1", result[0].NameLocale["zh_CN"])
	assert.Equal(t, "Mirror1", result[0].NameLocale["en_US"])
}

func TestToMirrorsSourceListSorted(t *testing.T) {
	m := mirrors{
		{Id: "low", Weight: 1, UrlHttp: "a.com"},
		{Id: "high", Weight: 100, UrlHttp: "b.com"},
		{Id: "mid", Weight: 50, UrlHttp: "c.com"},
	}
	result := toMirrorsSourceList(m)
	assert.Equal(t, "high", result[0].Id)
	assert.Equal(t, "mid", result[1].Id)
	assert.Equal(t, "low", result[2].Id)
}

func TestToMirrorsSourceListEmpty(t *testing.T) {
	result := toMirrorsSourceList(mirrors{})
	assert.Nil(t, result)
}

func TestMirrorSourceFields(t *testing.T) {
	m := mirrors{
		{Id: "m1", Name: "Test", Weight: 10, UrlHttps: "test.com", Country: "CN", AdjustDelay: 5},
	}
	result := toMirrorsSourceList(m)
	assert.Len(t, result, 1)
	s := system.MirrorSource{
		Id:          "m1",
		Name:        "Test",
		Weight:      10,
		Country:     "CN",
		AdjustDelay: 5,
		Url:         "https://test.com",
	}
	assert.Equal(t, s.Id, result[0].Id)
	assert.Equal(t, s.Name, result[0].Name)
	assert.Equal(t, s.Weight, result[0].Weight)
	assert.Equal(t, s.Country, result[0].Country)
	assert.Equal(t, s.AdjustDelay, result[0].AdjustDelay)
	assert.Equal(t, s.Url, result[0].Url)
}
