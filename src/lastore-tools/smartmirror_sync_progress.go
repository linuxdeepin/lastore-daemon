/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
package main

import "net/http"
import "time"
import "encoding/json"
import "fmt"
import "github.com/apcera/termtables"

type URLChecker struct {
	workQueue   chan string
	workerQueue chan chan string
	result      map[string]chan *URLCheckResult
	nDone       int
}
type URLCheckResult struct {
	URL     string
	Result  bool
	Latency time.Duration
}

func (c *URLChecker) Check(urls ...string) {
	for _, url := range urls {
		c.result[url] = make(chan *URLCheckResult, 1)
	}
}

func (c *URLChecker) Result(url string) *URLCheckResult {
	return <-c.result[url]
}

func NewURLChecker(thread int) *URLChecker {
	c := &URLChecker{
		workerQueue: make(chan chan string),
		result:      make(map[string]chan *URLCheckResult),
	}

	for i := 0; i < thread; i++ {
		go func() {
			worker := make(chan string)
			for {
				c.workerQueue <- worker
				select {
				case url := <-worker:
					r := CheckURLExists(url)
					c.nDone = c.nDone + 1
					c.result[url] <- r
					fmt.Printf("\r\n%0.1f%%  %q --> %v %v",
						float64(c.nDone)/float64(len(c.result))*100,
						url, r.Latency, r.Result)
					<-time.After(time.Millisecond * 100)
				}
			}
		}()
	}
	return c
}
func (u *URLChecker) Wait() {
	for url := range u.result {
		worker := <-u.workerQueue
		worker <- url
	}
	//	u.Stop()
}

func CheckURLExists(url string) *URLCheckResult {
	n := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		return &URLCheckResult{url, false, time.Since(n)}
	}
	defer resp.Body.Close()

	switch resp.StatusCode / 100 {
	case 4, 5:
		return &URLCheckResult{url, false, time.Since(n)}
	case 3, 2, 1:
		return &URLCheckResult{url, true, time.Since(n)}
	}
	return &URLCheckResult{url, false, time.Since(n)}
}

func ParseIndex(indexUrl string) ([]string, time.Time, error) {
	resp, err := http.Get(indexUrl)
	if err != nil {
		fmt.Println("E:", resp)
		return nil, time.Now(), err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	var lines []string
	err = d.Decode(&lines)

	t, e := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	if e != nil {
		fmt.Println("W:", e)
	}
	return lines, t, err
}

type MirrorInfo struct {
	Name        string
	Support2014 bool
	Support2015 bool
	Progress    float64
	Latency     time.Duration
	failedURLs  []string
}

func (MirrorInfo) String() {
	fmt.Sprint("%s 2014:%s")
}

func ShowMirrorInfos(infos []MirrorInfo, rd time.Time) {
	termtables.EnableUTF8PerLocale()

	t := termtables.CreateTable()
	t.AddHeaders("Name", "2014", "2015", "Latency", "Progress")
	title := fmt.Sprintf("Release at: %v  | report after %v",
		rd.Format(time.ANSIC),
		time.Now().Sub(rd))
	t.AddTitle(title)

	sym := map[bool]string{
		true:  "✔",
		false: "✖",
	}
	for _, info := range infos {
		name := info.Name
		if len(name) > 47 {
			name = name[0:47] + "..."
		}
		t.AddRow(name,
			sym[info.Support2014],
			sym[info.Support2015],
			fmt.Sprintf("%5.0fms", info.Latency.Seconds()*1000),
			fmt.Sprintf("%7.0f%%", info.Progress*100),
		)
	}

	fmt.Println("\n")
	fmt.Println(t.Render())
}

func u2014(server string) string {
	return appendSuffix(server, "/") + "dists/trusty/Release"
}
func u2015(server string) string {
	return appendSuffix(server, "/") + "dists/unstable/Release"
}
func uGuards(server string, guards []string) []string {
	var r []string
	// Just need precise of 5%. (Currently has 1%)
	for i, g := range guards {
		if i%5 == 0 {
			r = append(r, appendSuffix(server, "/")+g)
		}
	}
	return r
}

func DetectServer(parallel int, indexUrl string, mlist []string) ([]MirrorInfo, time.Time) {
	index, rd, err := ParseIndex(indexUrl)
	if err != nil || len(index) == 0 {
		fmt.Println("E:", err)
		return nil, rd
	}

	checker := NewURLChecker(parallel)
	for _, s := range mlist {
		checker.Check(u2014(s))
		checker.Check(u2015(s))
		checker.Check(uGuards(s, index)...)
	}
	checker.Wait()

	var r []MirrorInfo
	for _, s := range mlist {
		info := MirrorInfo{Name: s}
		p := 0
		guards := uGuards(s, index)
		var latency time.Duration
		for _, u := range guards {
			r := checker.Result(u)
			if r.Result {
				p = p + 1
			} else {
				info.failedURLs = append(info.failedURLs, r.URL)
			}
			latency = latency + r.Latency
		}

		info.Progress = float64(p) / float64(len(guards))
		info.Support2014 = checker.Result(u2014(s)).Result
		info.Support2015 = checker.Result(u2015(s)).Result
		info.Latency = time.Duration(int64(latency.Nanoseconds() / int64(len(guards))))
		r = append(r, info)
	}
	return r, rd
}
