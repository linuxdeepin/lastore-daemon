// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"sync"
	"time"
)

// Quality mean mirror can access and delay
// When AccessCount >=5, that mirror can be judge,
// If FailedCount >=3. that mirror in blacklist
// If not in blacklist, sort by AverageDelay
type Quality struct {
	DetectCount  int `json:"detect_count"`
	AccessCount  int `json:"access_count"`
	FailedCount  int `json:"failed_count"`
	AverageDelay int `json:"average_delay"`
}

// Report record mirror request status
type Report struct {
	Mirror     string
	URL        string
	Delay      time.Duration
	Failed     bool
	StatusCode int // http status code
}

func (r *Report) String() string {
	return fmt.Sprintf("%v %v %v %v", r.Mirror, r.Failed, r.Delay, r.StatusCode)
}

// QualityMap store all mirror quality status
type QualityMap map[string]*Quality

// MirrorQuality read mirror visit report and update QualityMap
type MirrorQuality struct {
	QualityMap
	adjustDelays map[string]int
	mux          sync.Mutex
	reportList   chan []Report
}

func (mq *MirrorQuality) updateQuality(r Report) {
	mq.mux.Lock()
	q := mq.getQuality(r.Mirror)
	if r.Failed {
		q.FailedCount++
	}
	totalDelay := q.AverageDelay*q.AccessCount + int(r.Delay.Nanoseconds()/1000/1000)
	q.AccessCount++
	q.AverageDelay = totalDelay / q.AccessCount
	mq.setQuality(r.Mirror, q)
	mq.mux.Unlock()
}

func (mq *MirrorQuality) getQuality(mirror string) *Quality {
	q, ok := mq.QualityMap[mirror]
	if ok {
		return q
	}
	mq.QualityMap[mirror] = &Quality{
		AverageDelay: 5000,
	}
	return mq.QualityMap[mirror]
}

// only for test
func (mq *MirrorQuality) setQuality(mirror string, q *Quality) {
	mq.QualityMap[mirror] = q
}

// loop select mirror while AccessCount < 5
// use 3+2 select
func (mq *MirrorQuality) detectSelectMirror(originMirrorList []string) []string {
	mq.mux.Lock()
	selectMirrorList := []string{}

	sortList := mq.sortSelectMirror(originMirrorList)
	lessAccessList := mq.lessAccessSelectMirror(originMirrorList)
	selectMirrorList = append(selectMirrorList, sortList...)

	// query map
	sortMap := make(map[string]string)
	for _, v := range sortList {
		sortMap[v] = v
	}
	for _, v := range lessAccessList {
		if _, ok := sortMap[v]; ok {
			continue
		}
		selectMirrorList = append(selectMirrorList, v)
		if len(selectMirrorList) >= 5 {
			break
		}
	}
	for _, v := range selectMirrorList {
		mq.QualityMap[v].DetectCount++
	}

	mq.mux.Unlock()
	return selectMirrorList
}

func (mq *MirrorQuality) lessAccessSelectMirror(originMirrorList []string) []string {
	lessAccessList := mq.mergeSort(originMirrorList, mq.selectLessAccess)
	max := 5
	if len(lessAccessList) < 5 {
		max = len(lessAccessList)
	}
	return lessAccessList[0:max]
}

func (mq *MirrorQuality) sortSelectMirror(originMirrorList []string) []string {
	sorted := mq.mergeSort(originMirrorList, mq.compare)
	max := 2
	if len(sorted) < 2 {
		max = len(sorted)
	}
	return sorted[0:max]
}

type compareHandler func(left, right string) bool

func (mq *MirrorQuality) selectLessAccess(left, right string) bool {
	lq := mq.getQuality(left)
	rq := mq.getQuality(right)
	return lq.DetectCount <= rq.DetectCount
}

// return true if left good than right
func (mq *MirrorQuality) compare(left, right string) bool {
	lq := mq.getQuality(left)
	rq := mq.getQuality(right)
	// WARNING: default value must be zero
	lAdjust := mq.adjustDelays[left]
	rAdjust := mq.adjustDelays[right]

	// TODO: 修复错误容限
	// equal (lq.FailedCount/lq.AccessCount < rq.FailedCount/rq.AccessCount)
	if lq.FailedCount*rq.AccessCount < rq.FailedCount*lq.AccessCount {
		return true
	}
	if lq.FailedCount*rq.AccessCount > rq.FailedCount*lq.AccessCount {
		return false
	}

	// lq.FailedCount/lq.AccessCount == rq.FailedCount/rq.AccessCount
	if (lq.AverageDelay + lAdjust) <= (rq.AverageDelay + rAdjust) {
		return true
	}
	return false
}

func (mq *MirrorQuality) mergeSort(originMirrorList []string, handler compareHandler) []string {
	if len(originMirrorList) < 2 {
		return originMirrorList
	}

	mid := len(originMirrorList) / 2
	left := originMirrorList[:mid]
	right := originMirrorList[mid:]
	return mq.merge(mq.mergeSort(left, handler), mq.mergeSort(right, handler), handler)
}

// good quality on left
func (mq *MirrorQuality) merge(left, right []string, handler compareHandler) []string {
	mergeList := []string{}

	for len(left) > 0 && len(right) > 0 {
		if handler(left[0], right[0]) {
			mergeList = append(mergeList, left[0])
			left = left[1:]
		} else {
			mergeList = append(mergeList, right[0])
			right = right[1:]
		}
	}

	for len(left) > 0 {
		mergeList = append(mergeList, left[0])
		left = left[1:]
	}
	for len(right) > 0 {
		mergeList = append(mergeList, right[0])
		right = right[1:]
	}
	return mergeList
}
