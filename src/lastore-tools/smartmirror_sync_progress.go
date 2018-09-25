/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import "net/http"
import "time"
import "encoding/json"
import "fmt"
import "github.com/apcera/termtables"
import "strings"
import "io"
import "internal/utils"

type URLChecker struct {
	workQueue   chan string
	workerQueue chan chan string
	result      map[string]chan *URLCheckResult
	nDone       int
}
type URLCheckResult struct {
	URL        string
	Result     bool
	ResultCode int
	StartTime  time.Time
	Latency    time.Duration
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
func (u *URLChecker) SendAllRequest() {
	for url := range u.result {
		worker := <-u.workerQueue
		worker <- url
	}
}

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
	defer resp.Body.Close()

	switch resp.StatusCode / 100 {
	case 4, 5:
		return &URLCheckResult{url, false, resp.StatusCode, n, time.Since(n)}
	case 3, 2, 1:
		return &URLCheckResult{url, true, resp.StatusCode, n, time.Since(n)}
	}
	return &URLCheckResult{url, false, resp.StatusCode, n, time.Since(n)}
}

func ParseIndex(indexUrl string) ([]string, error) {
	f, err := utils.OpenURL(indexUrl)
	if err != nil {
		return nil, err
	}
	defer f.Close()

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

func SaveMirrorInfos(infos []MirrorInfo, w io.Writer) error {
	return json.NewEncoder(w).Encode(infos)
}

func ShowMirrorInfos(infos []MirrorInfo) {
	termtables.EnableUTF8PerLocale()

	t := termtables.CreateTable()
	t.AddHeaders("Name", "2014", "Latency", "2015", "LastSync")
	t.AddTitle(fmt.Sprintf("Report at %v", time.Now()))

	sym := map[bool]string{
		true:  "✔",
		false: "✖",
	}
	for _, info := range infos {
		name := info.Name
		if len(name) > 47 {
			name = name[0:47] + "..."
		}
		var lm string = info.LastSync.Format(time.ANSIC)
		if info.LastSync.IsZero() {
			lm = "?"
		}
		t.AddRow(name,
			sym[info.Support2014],
			fmt.Sprintf("%5.0fms", info.Latency.Seconds()*1000),
			fmt.Sprintf("%7.0f%%", info.Progress*100),
			lm,
		)
	}

	fmt.Print("\n\n")
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

func DetectServer(parallel int, indexName string, official string, mlist []string) []MirrorInfo {
	indexUrl := appendSuffix(official, "/") + indexName
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
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("E:", resp)
		return time.Time{}
	}
	defer resp.Body.Close()
	t, e := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	if e != nil {
		fmt.Println("\nfetchLastSync:", url, e)
	}
	return t
}
