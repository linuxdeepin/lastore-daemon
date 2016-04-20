/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import (
	"internal/utils"
	"sync"
	"time"
)

type FileChecker struct {
	mirrors map[string]MirrorType
}

func NewFileChecker(mirrors map[string]MirrorType) *FileChecker {
	return &FileChecker{
		mirrors: mirrors,
	}
}

func (checker *FileChecker) Check(filename string, timeout time.Duration) (string, chan []*URLCheckResult) {
	best, results := checker.choose(filename)

	select {
	case c := <-best:
		return c, results
	case <-time.After(timeout):
		return checker.officialMirror(), results
	}
}

// Choose the proper server in mirrors.
// timeout specifies the maximum duration the Do should
// block for waiting.
// But the network connect wouldn't closed until it
// record the result.
func (checker *FileChecker) sendRequest(filename string) (chan *URLCheckResult, chan *URLCheckResult) {
	ps := checker.preferenceMirrors()
	ss := checker.standbyMirrors()

	rPreference := make(chan *URLCheckResult, len(ps))
	rStandby := make(chan *URLCheckResult, len(ss))

	go func() {
		var w sync.WaitGroup
		for _, m := range ps {
			w.Add(1)
			go func(url string) {
				rPreference <- CheckURLExists(url)
				w.Done()
			}(utils.AppendSuffix(m, "/") + filename)
		}
		w.Wait()
		close(rPreference)
	}()
	go func() {
		var w sync.WaitGroup
		for _, m := range ss {
			w.Add(1)
			go func(url string) {
				rStandby <- CheckURLExists(url)
				w.Done()
			}(utils.AppendSuffix(m, "/") + filename)
		}
		w.Wait()
		close(rStandby)
	}()

	return rPreference, rStandby
}

func (checker *FileChecker) choose(filename string) (chan string, chan []*URLCheckResult) {
	rPreference, rStandby := checker.sendRequest(filename)

	best, results := make(chan string, 1), make(chan []*URLCheckResult, 1)
	go func() {
		hasBest := false
		var rs []*URLCheckResult
		for r := range rPreference {
			server := r.URL[:len(r.URL)-len(filename)]
			if !hasBest && r.Result {
				hasBest = true
				best <- server
			}
			r.URL = server
			rs = append(rs, r)
		}

		for r := range rStandby {
			server := r.URL[:len(r.URL)-len(filename)]
			if !hasBest && r.Result {
				hasBest = true
				best <- server
			}
			r.URL = server
			rs = append(rs, r)
		}
		if !hasBest {
			hasBest = true
			best <- checker.officialMirror()
		}
		results <- rs
	}()

	return best, results
}
func (d *FileChecker) standbyMirrors() []string {
	var r []string
	for m, t := range d.mirrors {
		if t == MirrorTypeStandBy {
			r = append(r, m)
		}
	}
	return r
}
func (d *FileChecker) preferenceMirrors() []string {
	var r []string
	for m, t := range d.mirrors {
		if t == MirrorTypePreference {
			r = append(r, m)
		}
	}
	return r
}
func (d *FileChecker) officialMirror() string {
	for m, t := range d.mirrors {
		if t == MirrorTypeOfficial {
			return m
		}
	}
	return "http://packages.deepin.com/deepin/"
}

type MirrorType int

const (
	MirrorTypeOfficial MirrorType = iota
	MirrorTypePreference
	MirrorTypeStandBy
)

func buildMirrors(official string, preference []string, standby []string) map[string]MirrorType {
	mirrors := make(map[string]MirrorType)

	for _, s := range standby {
		mirrors[utils.AppendSuffix(s, "/")] = MirrorTypeStandBy
	}
	for _, s := range preference {
		mirrors[utils.AppendSuffix(s, "/")] = MirrorTypePreference
	}
	mirrors[utils.AppendSuffix(official, "/")] = MirrorTypeOfficial

	return mirrors
}
