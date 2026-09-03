// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStripPort_NoPort(t *testing.T) {
	result := stripPort("example.com")
	assert.Equal(t, "example.com", result)
}

func TestStripPort_WithPort(t *testing.T) {
	result := stripPort("example.com:8080")
	assert.Equal(t, "example.com", result)
}

func TestStripPort_IPv6(t *testing.T) {
	result := stripPort("[::1]:8080")
	assert.Equal(t, "::1", result)
}

func TestStripPort_IPv6NoPort(t *testing.T) {
	result := stripPort("[::1]")
	assert.Equal(t, "::1", result)
}

func TestGetQuality_ExistingMirror(t *testing.T) {
	mq := &MirrorQuality{
		QualityMap: QualityMap{
			"mirror1": {AverageDelay: 100},
		},
	}
	q := mq.getQuality("mirror1")
	assert.NotNil(t, q)
	assert.Equal(t, 100, q.AverageDelay)
}

func TestGetQuality_NewMirror(t *testing.T) {
	mq := &MirrorQuality{
		QualityMap: QualityMap{},
	}
	q := mq.getQuality("mirror2")
	assert.NotNil(t, q)
	assert.Equal(t, 5000, q.AverageDelay)
	assert.Contains(t, mq.QualityMap, "mirror2")
}

func TestSetQuality(t *testing.T) {
	mq := &MirrorQuality{
		QualityMap: QualityMap{},
	}
	q := &Quality{AverageDelay: 200, AccessCount: 5}
	mq.setQuality("mirror3", q)
	assert.Equal(t, q, mq.QualityMap["mirror3"])
}

func TestUpdateQuality_Success(t *testing.T) {
	mq := &MirrorQuality{
		QualityMap: QualityMap{},
	}
	r := Report{
		Mirror: "mirror1",
		Delay:  50 * time.Millisecond,
		Failed: false,
	}
	mq.updateQuality(r)
	q := mq.QualityMap["mirror1"]
	assert.NotNil(t, q)
	assert.Equal(t, 1, q.AccessCount)
	assert.Equal(t, 0, q.FailedCount)
	assert.Equal(t, 50, q.AverageDelay)
}

func TestUpdateQuality_Failed(t *testing.T) {
	mq := &MirrorQuality{
		QualityMap: QualityMap{},
	}
	r := Report{
		Mirror: "mirror1",
		Delay:  100 * time.Millisecond,
		Failed: true,
	}
	mq.updateQuality(r)
	q := mq.QualityMap["mirror1"]
	assert.NotNil(t, q)
	assert.Equal(t, 1, q.AccessCount)
	assert.Equal(t, 1, q.FailedCount)
}

func TestUpdateQuality_Multiple(t *testing.T) {
	mq := &MirrorQuality{
		QualityMap: QualityMap{},
	}
	mq.updateQuality(Report{Mirror: "m1", Delay: 100 * time.Millisecond})
	mq.updateQuality(Report{Mirror: "m1", Delay: 200 * time.Millisecond})
	q := mq.QualityMap["m1"]
	assert.Equal(t, 2, q.AccessCount)
	assert.Equal(t, 150, q.AverageDelay)
}

func TestSortSelectMirror_Empty(t *testing.T) {
	mq := &MirrorQuality{
		QualityMap:    QualityMap{},
		adjustDelays:  map[string]int{},
	}
	result := mq.sortSelectMirror([]string{})
	assert.Empty(t, result)
}

func TestSortSelectMirror_SingleMirror(t *testing.T) {
	mq := &MirrorQuality{
		QualityMap: QualityMap{
			"m1": {AverageDelay: 100, AccessCount: 10, FailedCount: 0},
		},
		adjustDelays: map[string]int{},
	}
	result := mq.sortSelectMirror([]string{"m1"})
	assert.Equal(t, []string{"m1"}, result)
}

func TestSortSelectMirror_SortedByQuality(t *testing.T) {
	mq := &MirrorQuality{
		QualityMap: QualityMap{
			"m1": {AverageDelay: 300, AccessCount: 10, FailedCount: 0},
			"m2": {AverageDelay: 100, AccessCount: 10, FailedCount: 0},
		},
		adjustDelays: map[string]int{},
	}
	result := mq.sortSelectMirror([]string{"m1", "m2"})
	assert.Equal(t, []string{"m2", "m1"}, result)
}
