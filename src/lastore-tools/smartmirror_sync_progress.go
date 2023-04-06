// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"
	"io"
	"net/http"
	"strings"
	"time"
)

// 目前这部分代码大部分工作不正常

type URLChecker struct {
	workerQueue chan chan string
	result      map[string]chan *URLCheckResult
	nDone       int
}
type URLCheckResult struct {
	URL        string
	Result     bool
	ResultCode int
	StartTime  time.Time
	Latency    time.Duration // 延时，其实是检查消耗时间
}

func (c *URLChecker) Check(urls ...string) {
	for _, url := range urls {
		if _, ok := c.result[url]; ok {
			panic(fmt.Sprintf("URLChecker try checking exists url %q", url))
		}
		c.result[url] = make(chan *URLCheckResult, 1)
	}
}

func (c *URLChecker) Result(url string) *URLCheckResult {
	r, ok := c.result[url]
	if !ok {
		panic(fmt.Sprintf("URLChecker try geting doesn't exists url %q", url))
	}
	defer delete(c.result, url)
	return <-r
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

				url := <-worker
				r := CheckURLExists(url)
				c.nDone = c.nDone + 1
				c.result[url] <- r
				fmt.Printf("\r\n%0.1f%%  %q --> %v %v",
					float64(c.nDone)/float64(len(c.result))*100,
					url, r.Latency, r.Result)
				<-time.After(time.Millisecond * 100)
			}
		}()
	}
	return c
}
func (u *URLChecker) SendAllRequest() {
	for url := range u.result {
		worker := <-u.workerQueue
		worker <- url
	}
}

// CheckURLExists 检查 url 的响应状态码
func CheckURLExists(url string) *URLCheckResult {
	n := time.Now().UTC()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &URLCheckResult{url, false, 0, n, time.Since(n)}
	}
	req.Header.Set("User-Agent", "lastore-tools")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &URLCheckResult{url, false, 0, n, time.Since(n)}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode / 100 {
	case 4, 5:
		return &URLCheckResult{url, false, resp.StatusCode, n, time.Since(n)}
	case 3, 2, 1:
		return &URLCheckResult{url, true, resp.StatusCode, n, time.Since(n)}
	}
	return &URLCheckResult{url, false, resp.StatusCode, n, time.Since(n)}
}

// ParseIndex 获取 indexUrl 的内容，然后过滤重复行。
func ParseIndex(indexUrl string) ([]string, error) {
	f, err := utils.OpenURL(indexUrl)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	d := json.NewDecoder(f)
	var lines []string
	err = d.Decode(&lines)

	// filter repeat lines.
	tmp := make(map[string]struct{})
	for _, line := range lines {
		tmp[line] = struct{}{}
	}
	var r []string
	for line := range tmp {
		r = append(r, line)
	}
	return r, err
}

type MirrorInfo struct {
	Name        string
	Support2014 bool
	Support2015 bool
	Progress    float64
	LastSync    time.Time
	Latency     time.Duration
	Detail      []URLCheckResult
}

// SaveMirrorInfos 把 infos 序列化为 JSON 格式，用 w 写入。
func SaveMirrorInfos(infos []MirrorInfo, w io.Writer) error {
	return json.NewEncoder(w).Encode(infos)
}

func u2014(server string) string {
	return appendSuffix(server, "/") + "dists/trusty/Release"
}
func u2015(server string) string {
	return appendSuffix(server, "/") + "dists/unstable/Release"
}

// uGuards 把 guards 列表中提出 1/5 的部分，和 server 组合为 url。
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

func DetectServer(parallel int, indexName string, official string, mlist []string) []MirrorInfo {
	indexUrl := appendSuffix(official, "/") + indexName
	// 获取 GUARD_INDEX 文件，这个文件是所有 GUARD 文件的索引，是 JSON 格式，字符串列表。
	// 每个 GUARD 文件命名成 __GUARD__ + unix时间戳，是空文件。
	index, err := ParseIndex(indexUrl)
	if err != nil || len(index) == 0 {
		fmt.Println("E:", err)
		return nil
	}
	mlist = append([]string{official}, mlist...)

	checker := NewURLChecker(parallel)
	for _, s := range mlist {
		checker.Check(u2014(s))
		checker.Check(u2015(s))
		checker.Check(uGuards(s, index)...)
	}
	checker.SendAllRequest()

	var infos []MirrorInfo
	for _, s := range mlist {
		info := MirrorInfo{
			Name:     s,
			LastSync: fetchLastSync(appendSuffix(s, "/") + indexName),
		}
		p := 0
		guards := uGuards(s, index)
		var latency time.Duration
		for _, u := range guards {
			r := checker.Result(u)
			if r.Result {
				p = p + 1
			}
			if strings.HasPrefix(r.URL, s) {
				info.Detail = append(info.Detail, *r)
			}
			latency = latency + r.Latency
		}

		info.Progress = float64(p) / float64(len(guards))
		info.Support2014 = checker.Result(u2014(s)).Result
		info.Support2015 = checker.Result(u2015(s)).Result
		info.Latency = time.Duration(int64(latency.Nanoseconds() / int64(len(guards))))
		infos = append(infos, info)
	}
	return infos
}

func fetchLastSync(url string) time.Time {
	// #nosec G107
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("E:", resp)
		return time.Time{}
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	t, e := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	if e != nil {
		fmt.Println("\nfetchLastSync:", url, e)
	}
	return t
}
