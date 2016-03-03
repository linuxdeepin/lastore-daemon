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
	"encoding/json"
	"fmt"
	"github.com/codegangsta/cli"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

var CMDSmartMirror = cli.Command{
	Name:  "smartmirror",
	Usage: `query of select mirrors`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "official",
			Value: "http://packages.deepin.com/deepin",
			Usage: "the official package repository",
		},
		cli.StringFlag{
			Name:  "mirrorlist,m",
			Value: "/var/lib/lastore/mirrors.json",
			Usage: "the list of mirrors, maintained by official",
		},
		cli.StringFlag{
			Name:  "db,d",
			Value: "/var/lib/lastore/history.db",
			Usage: "the db to store the history information",
		},
		cli.BoolFlag{
			Name:  "quiet,q",
			Usage: "slient mode, only produces necessary output",
		},
		cli.IntFlag{
			Name:  "interval,i",
			Value: 300,
			Usage: "minimum interval in seconds allow for rechecking failed mirror",
		},
		cli.IntFlag{
			Name:  "parallel,p",
			Value: 5,
			Usage: "maximum http connections allow for detecting to take",
		},
	},
	Subcommands: []cli.Command{
		{
			Name:   "choose",
			Usage:  "detect who will serve the pkg",
			Action: SubmainMirrorChoose,
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "timeout,t",
					Value: 5,
					Usage: "maximum time in seconds allow for detecting to take",
				},
				cli.StringSliceFlag{
					Name:  "permanent,p",
					Usage: "this mirrors will be detected no matter what they status",
				},
			},
		},
		{
			Name: "stats",
			Usage: `show the history of serving
     ✓ and ★ indicate the candidates in next mirror selecting.
     But ★ also indicate the mirror was unworkable in preview detecting.`,
			Action: SubmainMirrorStats,
		},
		{
			Name:   "server_stats",
			Usage:  "report the status of all knonw mirrors",
			Action: SubmainMirrorSynProgress,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "index,i",
					Value: ".__GUARD__INDEX__",
					Usage: "the guard index path in official",
				},
				cli.BoolFlag{
					Name:  "list,l",
					Usage: "only list knonwn mirrors",
				},
			},
		},
		{
			Name:   "report",
			Usage:  "report the choiced server for serving filename",
			Action: SubmainMirrorReport,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "server,s",
					Usage: "the chocied server for serving filename",
				},
			},
		},
	},
}

func SubmainMirrorSynProgress(c *cli.Context) {
	indexName := c.String("index")
	official := c.Parent().String("official")

	onlyList := c.Bool("list")
	n := c.Parent().Int("parallel")

	if onlyList {
		mlist, _ := getMirrorList(c.Parent().String("mirrorlist"))
		for _, m := range mlist {
			fmt.Println(m)
		}
		return
	}

	mlist := c.Args()
	if len(mlist) == 0 {
		mlist, _ = getMirrorList(c.Parent().String("mirrorlist"))
	}

	ShowMirrorInfos(DetectServer(n, indexName, official, mlist))
}

func SubmainMirrorStats(c *cli.Context) {
	parallel := c.Parent().Int("parallel")
	interval := time.Second * time.Duration(c.Parent().Int("interval"))
	db := DB{c.Parent().String("db")}
	cache, err := db.LoadMirrorCache()
	if err != nil {
		fmt.Printf("E:%v\n", err)
	}
	fmt.Println(cache.ShowStats(parallel, interval))
}

func ShowBest(url string) {
	fmt.Println(url)
}
func ShowBestOnError(url string, err error) {
	ShowBest(url)
	fmt.Printf("E:%v\n", err)
	os.Exit(-1)
}

func SubmainMirrorChoose(c *cli.Context) {
	signal.Ignore(syscall.SIGPIPE, syscall.SIGIO)

	filename := c.Args().First()
	official := appendSuffix(c.Parent().String("official"), "/") + filename
	dbPath := c.Parent().String("db")
	parallel := c.Parent().Int("parallel")
	interval := time.Second * time.Duration(c.Parent().Int("interval"))
	timeout := time.Second * time.Duration(c.Int("timeout"))

	var permanent []string
	for _, i := range c.StringSlice("permanent") {
		permanent = append(permanent, appendSuffix(i, "/"))
	}

	if parallel < 1 {
		fmt.Printf("At least two http connections for detecting, but there has only %d\n", parallel)
		os.Exit(-1)
	}
	mlist, err := getMirrorList(c.Parent().String("mirrorlist"))
	if err != nil {
		ShowBestOnError(official, err)
	}

	db := DB{dbPath}
	cache, err := db.LoadMirrorCache()
	if err != nil {
		ShowBest(official)
	}

	cacheFiltered, newServers := cache.Filter(mlist)
	var candidate []string
	for _, v := range cacheFiltered.Find(parallel, interval) {
		candidate = append(candidate, v.Name)
	}
	candidate = append(candidate, newServers...)
	candidate = append(candidate, permanent...)

	r := func(s string, t time.Duration, h bool, u bool) error {
		return db.Record(s, t, h, u)
	}
	d := NewDetector(filename, timeout, r, c.GlobalBool("debug"))
	d.Do(candidate)
	d.WaitAll()
}

type DetectRecorder func(server string, timeout time.Duration, succeeded bool, used bool) error
type Detector struct {
	FileName string

	debug  bool
	client *http.Client
	waiter sync.WaitGroup

	best     string
	recorder DetectRecorder
}

func NewDetector(filename string, timeout time.Duration, recorder DetectRecorder, debug bool) *Detector {
	return &Detector{
		debug:    debug,
		FileName: filename,
		client:   &http.Client{Timeout: timeout},
		recorder: recorder,
	}
}

func (d *Detector) Do(mirrors []string) {
	result := make(chan string, len(mirrors))
	doneList := make(map[string]struct{})
	for _, m := range mirrors {
		if _, ok := doneList[m]; ok {
			continue
		}
		doneList[m] = struct{}{}

		d.waiter.Add(1)
		go d.doOne(m, result)
	}
}

func (d *Detector) WaitAll() {
	d.waiter.Wait()
}

func (d *Detector) doOne(server string, result chan string) {
	begin := time.Now()
	url := appendSuffix(server, "/") + d.FileName

	s, ok := handleRequest(d.client, buildRequest(nil, "GET", url))

	t := time.Now().Sub(begin)
	if d.best == "" && ok {
		d.best = s
		ShowBest(d.best)
	}
	if d.recorder != nil {
		err := d.recorder(server, t, ok, s == d.best && ok)
		if err != nil {
			debugPrint("Record failed :%v\n", err)
		}
	}

	if d.debug {
		debugPrint("Use %v check %q --> %v\n", t, url, ok)
	}

	result <- s
	d.waiter.Done()
}

func SubmainMirrorReport(c *cli.Context) {
	filename := c.Args().First()
	if filename == "" {
		cli.ShowCommandHelp(c, "report")
		os.Exit(-1)
	}

	const DetectVersion = "detector/0.1.1 " + runtime.GOARCH

	var userAgent string
	r, _ := exec.Command("lsb_release", "-ds").Output()
	if len(r) == 0 {
		userAgent = DetectVersion + " " + "deepin unknown"
	} else {
		userAgent = DetectVersion + " " + strings.TrimSpace(string(r))
	}
	bs, _ := ioutil.ReadFile("/etc/machine-id")

	head := map[string]string{
		"User-Agent": userAgent,
		"MID":        strings.TrimSpace(string(bs)),
		"Mirror":     appendSuffix(c.String("server"), "/"),
	}

	req := buildRequest(head, "HEAD", appendSuffix(c.Parent().String("official"), "/")+filename)
	s, ok := handleRequest(nil, req)
	if c.GlobalBool("debug") {
		debugPrint("Report: %q Hint : %v\n", s, ok)
		d, err := httputil.DumpRequest(req, true)
		debugPrint(">>>>>>>>>>>>>> Detail <<<<<<<<<<<<<<\n")
		debugPrint(string(d), err, "\n")
	}

}

func buildRequest(header map[string]string, method string, url string) *http.Request {
	r, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil
	}
	for k, v := range header {
		r.Header.Set(k, v)
	}
	return r
}

func handleRequest(c *http.Client, r *http.Request) (string, bool) {
	if c == nil {
		c = &http.Client{}
	}
	if r == nil {
		return "", false
	}
	resp, err := c.Do(r)
	if err != nil {
		return "", false
	}
	resp.Body.Close()

	switch resp.StatusCode / 100 {
	case 4, 5:
		return "", false
	case 3:
		u, err := resp.Location()
		if err != nil {
			return r.URL.String(), true
		}
		return u.String(), true
	case 2, 1:
		return r.URL.String(), true
	default:
		return "", true
	}
}

func appendSuffix(r string, suffix string) string {
	if strings.HasSuffix(r, suffix) {
		return r
	}
	return r + suffix
}

func getMirrorList(p string) ([]string, error) {
	if strings.HasPrefix(p, "http://") {
		ms, err := LoadMirrorSources(p)
		if err != nil {
			return nil, err
		}

		var r []string
		for _, m := range ms {
			r = append(r, m.Url)
		}
		return r, nil
	}

	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	d := json.NewDecoder(f)
	var raw []struct {
		Url string `json:"url"`
	}
	err = d.Decode(&raw)

	var r []string
	for _, u := range raw {
		r = append(r, u.Url)
	}
	return r, err
}
