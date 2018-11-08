package main

import (
	"math/rand"
	"sync"
	"time"
)

// Quality mean mirror can access and delay
// When AccessCount >=5, that mirror can be judge,
// If FailedCount >=3. that mirror in blacklist
// If not in blacklist, sort by AverageDelay
type Quality struct {
	AccessCount  int
	FailedCount  int
	AverageDelay int // ms
}

// Report record mirror request status
type Report struct {
	Mirror string
	URL    string
	Delay  time.Duration
	Failed bool
}

// QualityMap store all mirror quality status
type QualityMap map[string]*Quality

// MirrorQuality read mirror visit report and update QualityMap
type MirrorQuality struct {
	QualityMap
	mux    sync.Mutex
	report chan Report
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
	randomList := mq.randomSelectMirror(originMirrorList)
	for _, v := range sortList {
		if _, ok := randomList[v]; ok {
			delete(randomList, v)
		}
	}
	selectMirrorList = append(selectMirrorList, sortList...)
	for _, v := range randomList {
		selectMirrorList = append(selectMirrorList, v)
		if len(selectMirrorList) >= 5 {
			break
		}
	}

	mq.mux.Unlock()
	return selectMirrorList
}

func (mq *MirrorQuality) randomSelectMirror(originMirrorList []string) map[string]string {
	randomList := make(map[string]string, 0)
	num := len(originMirrorList)
	for i := 0; i < 5; i++ {
		mirror := originMirrorList[rand.Intn(num)]
		randomList[mirror] = mirror
	}
	return randomList
}

func (mq *MirrorQuality) sortSelectMirror(originMirrorList []string) []string {
	sorted := mq.mergeSort(originMirrorList)
	return sorted[0:2]
}

// return true if left good than right
func (mq *MirrorQuality) compare(left, right string) bool {
	lq := mq.getQuality(left)
	rq := mq.getQuality(right)

	// equal (lq.FailedCount/lq.AccessCount < rq.FailedCount/rq.AccessCount)
	if lq.FailedCount*rq.AccessCount < rq.FailedCount*lq.AccessCount {
		return true
	}
	if lq.FailedCount*rq.AccessCount > rq.FailedCount*lq.AccessCount {
		return false
	}

	// lq.FailedCount/lq.AccessCount == rq.FailedCount/rq.AccessCount
	if lq.AverageDelay <= rq.AverageDelay {
		return true
	}
	return false
}

func (mq *MirrorQuality) mergeSort(originMirrorList []string) []string {
	if len(originMirrorList) < 2 {
		return originMirrorList
	}

	mid := len(originMirrorList) / 2
	left := originMirrorList[:mid]
	right := originMirrorList[mid:]
	return mq.merge(mq.mergeSort(left), mq.mergeSort(right))
}

// good quality on left
func (mq *MirrorQuality) merge(left, right []string) []string {
	mergeList := []string{}

	for len(left) > 0 && len(right) > 0 {
		if mq.compare(left[0], right[0]) {
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
