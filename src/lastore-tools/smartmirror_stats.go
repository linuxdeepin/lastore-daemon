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
	"fmt"
	"github.com/apcera/termtables"
	"github.com/boltdb/bolt"
	"os"
	"sort"
	"strconv"
	"time"
)

var (
	// negative value meaning last checking is failed;
	// positive value meaning last checking is succeeded;
	// the abs(value) meaning max times in this state
	HealthBucket = ([]byte)("health")

	// last checking timestamp
	LastCheckTimeBucket = ([]byte)("last_check_time")

	// last checking time delay
	LatencyBucket = ([]byte)("latency")

	// total of selected
	UsedCountBucket = ([]byte)("used_count")

	// total of failed detecting
	FailedCountBucket = ([]byte)("failed_count")

	// total of succeeded detecting
	SucceededCountBucket = ([]byte)("succeeded_count")
)

type DB struct {
	dbPath string
}

func (db DB) Record(server string, latency time.Duration, hit bool, used bool) error {
	core, err := bolt.Open(db.dbPath, 0644, nil)
	if err != nil {
		return err
	}
	defer core.Close()

	return core.Update(func(tx *bolt.Tx) error {
		keyS := ([]byte)(server)

		b, err := tx.CreateBucketIfNotExists(LatencyBucket)
		if err != nil {
			return err
		}
		b.Put(keyS, ([]byte)(latency.String()))

		b, err = tx.CreateBucketIfNotExists(LastCheckTimeBucket)
		if err != nil {
			return err
		}
		t, _ := time.Now().MarshalBinary()
		b.Put(keyS, t)

		b, err = tx.CreateBucketIfNotExists(HealthBucket)
		nS, _ := strconv.Atoi(string(b.Get((keyS))))
		if hit {
			if nS < 0 {
				nS = 0
			}
			b.Put(keyS, ([]byte)(strconv.Itoa(nS+1)))
		} else {
			if nS > 0 {
				nS = 0
			}
			b.Put(keyS, ([]byte)(strconv.Itoa(nS-1)))
		}

		if hit {
			b, err = tx.CreateBucketIfNotExists(SucceededCountBucket)
		} else {
			b, err = tx.CreateBucketIfNotExists(FailedCountBucket)
		}
		if err != nil {
			return err
		}
		nS, _ = strconv.Atoi(string(b.Get((keyS))))
		b.Put(keyS, ([]byte)(strconv.Itoa(nS+1)))

		if used {
			b, err = tx.CreateBucketIfNotExists(UsedCountBucket)
			if err != nil {
				return err
			}
			nS, _ = strconv.Atoi(string(b.Get((keyS))))
			b.Put(keyS, ([]byte)(strconv.Itoa(nS+1)))
		}

		return nil
	})
}

func (db DB) LoadMirrorCache() (MirrorCache, error) {
	core, err := bolt.Open(db.dbPath, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return nil, fmt.Errorf("Hasn't any history! Please go back after play for a while.")
		}
		return nil, err
	}
	defer core.Close()

	r := make(map[string]*MirrorCacheInfo)

	forEach := func(b *bolt.Bucket, seter func(k string, v []byte) error) error {
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			name := string(k)
			if _, ok := r[name]; !ok {
				r[name] = &MirrorCacheInfo{Name: name}
			}
			return seter(name, v)

		})
	}

	err = core.View(func(tx *bolt.Tx) error {
		err := forEach(tx.Bucket(FailedCountBucket), func(name string, v []byte) error {
			var err error
			r[name].FailedCount, err = strconv.Atoi(string(v))
			return err
		})
		if err != nil {
			return err
		}

		err = forEach(tx.Bucket(SucceededCountBucket), func(name string, v []byte) error {
			r[name].SucceededCount, err = strconv.Atoi(string(v))
			return err
		})
		if err != nil {
			return err
		}

		err = forEach(tx.Bucket(UsedCountBucket), func(name string, v []byte) error {
			r[name].UsedCount, err = strconv.Atoi(string(v))
			return err
		})
		if err != nil {
			return err
		}

		err = forEach(tx.Bucket(LatencyBucket), func(name string, v []byte) error {
			r[name].Latency, err = time.ParseDuration(string(v))
			return err
		})
		if err != nil {
			return err
		}

		err = forEach(tx.Bucket(HealthBucket), func(name string, v []byte) error {
			n, err := strconv.Atoi(string(v))
			r[name].Health = n
			return err
		})
		if err != nil {
			return err
		}

		err = forEach(tx.Bucket(LastCheckTimeBucket), func(name string, v []byte) error {
			t := &time.Time{}
			if err := t.UnmarshalBinary(v); err != nil {
				return err
			}
			r[name].LastCheckTime = *t
			return nil
		})
		return err

	})
	var cache MirrorCache
	for _, v := range r {
		cache = append(cache, v)
	}

	return cache, err
}

type MirrorCache []*MirrorCacheInfo

type MirrorCacheInfo struct {
	Name          string
	Health        int
	LastCheckTime time.Time
	Latency       time.Duration

	FailedCount    int
	SucceededCount int
	UsedCount      int
}

const (
	Red   = "0;31"
	Blue  = "0;34"
	White = "1;37"
)

func ColorSprintf(color string, fmtStr string, args ...interface{}) string {
	return fmt.Sprintf("\033["+color+"m"+fmtStr+"\033[0m", args...)
}

func (c MirrorCache) Len() int      { return len(c) }
func (c MirrorCache) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

func (c MirrorCache) ShowStats(parallel int, interval time.Duration) string {
	count := 0
	for _, v := range c {
		count = count + v.UsedCount
	}
	best := make(map[string]bool)
	for _, v := range append(c.Find(parallel, interval)) {
		best[v.Name] = v.Health >= 0
	}

	termtables.EnableUTF8PerLocale()

	t := termtables.CreateTable()
	t.AddHeaders("Name", "Health", "Latency", "Selected", "Hit Ratio", "Check Time")

	sort.Sort(sort.Reverse(_MirrorByLatency{c}))
	for _, v := range c {
		name := v.Name
		if v, ok := best[name]; ok {
			if v {
				name = "✓ " + name
			} else {
				name = "★ " + name
			}
		} else {
			name = "  " + name
		}
		if len(name) > 47 {
			name = string(name[:47]) + "..."
		}

		duration := time.Since(v.LastCheckTime).Seconds()
		health := fmt.Sprintf("%v", v.Health)
		if v.Health < 0 {
			health = ColorSprintf(Red, health)
		}
		t.AddRow(name,
			health,
			fmt.Sprintf("%5.0fms", v.Latency.Seconds()*1000),
			fmt.Sprintf("%.1f%%", float64(v.UsedCount)*100/float64(count)),
			fmt.Sprintf("%d/%d(%0.1f%%)", v.SucceededCount, v.FailedCount, float64(v.SucceededCount)*100/float64(v.SucceededCount+v.FailedCount)),
			fmt.Sprintf("%.0fs ago", duration),
		)
	}
	return t.Render()
}

type _MirrorByLatency struct{ MirrorCache }
type _MirrorByFailed struct{ MirrorCache }
type _MirrorByHealth struct{ MirrorCache }

func (m _MirrorByLatency) Less(i, j int) bool {
	return m.MirrorCache[i].Latency > m.MirrorCache[j].Latency
}
func (m _MirrorByHealth) Less(i, j int) bool {
	return m.MirrorCache[i].Health < m.MirrorCache[j].Health
}
func (m _MirrorByFailed) Less(i, j int) bool {
	return m.MirrorCache[i].Health > m.MirrorCache[j].Health
}

// Bests return the best mirror list.
func (c MirrorCache) Bests(n int) MirrorCache {
	sort.Sort(sort.Reverse(_MirrorByLatency{c}))

	var r MirrorCache

	for _, info := range c {
		if info.Health < 0 {
			continue
		}
		r = append(r, info)
		if len(r) >= n {
			break
		}
	}
	return r
}

func (c MirrorCache) Standby(n int, begin time.Time, d time.Duration) MirrorCache {
	sort.Sort(sort.Reverse(_MirrorByFailed{c}))
	var r MirrorCache
	for _, info := range c {
		if info.Health >= 0 || begin.Sub(info.LastCheckTime) < d {
			continue
		}
		r = append(r, info)
	}
	sort.Sort(sort.Reverse(_MirrorByHealth{r}))
	if len(r) <= n {
		return r
	} else {
		return r[0:n]
	}
}

// Filter return MirrorCcache in whiteList and new servers
func (c MirrorCache) Filter(whiteList []string) (MirrorCache, []string) {
	checker := make(map[string]bool)
	for _, w := range whiteList {
		checker[w] = true
	}

	var r1 MirrorCache
	for _, i := range c {
		if checker[i.Name] {
			r1 = append(r1, i)
			delete(checker, i.Name)
		}
	}

	var r2 []string
	for _, i := range whiteList {
		if checker[i] {
			r2 = append(r2, i)
		}
	}
	return r1, r2
}

func (c MirrorCache) Find(n int, d time.Duration) MirrorCache {
	if n < 2 {
		panic("At least 2")
	}
	bests := c.Bests(n - 1)
	r := append(bests, c.Standby(n-len(bests), time.Now(), d)...)
	return r
}
