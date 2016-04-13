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
	"internal/utils"
	"os"
	"os/signal"
	"strings"
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
					Name:  "preference,p",
					Usage: "mirrors will be detected no matter what they status",
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
				cli.StringFlag{
					Name:  "export,e",
					Value: "result.json",
					Usage: "the file to save the results in json format",
				},
			},
		},
	},
}

func SubmainMirrorStats(c *cli.Context) {
	parallel := c.Parent().Int("parallel")
	interval := time.Second * time.Duration(c.Parent().Int("interval"))
	db := MirrorRecordDB{c.Parent().String("db")}
	cache, err := db.LoadMirrorCache()
	if err != nil {
		fmt.Printf("E:%v\n", err)
	}
	fmt.Println(cache.ShowStats(parallel, interval))
}

func SubmainMirrorChoose(c *cli.Context) {
	// NOTE: Don't write anything to stdout before ReportChoosedServer,
	// Because apt-get would use first line of SubmainMirrorChoose as
	// the best server url.

	filename := c.Args().First()

	db := MirrorRecordDB{c.Parent().String("db")}

	standby, err := LoadAutochooseMirrors(
		c.Parent().Int("parallel"),
		time.Second*time.Duration(c.Parent().Int("interval")),
		db,
		c.Parent().String("mirrorlist"),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "W: ", err)
	}

	official := utils.AppendSuffix(c.Parent().String("official"), "/")
	mirrors := buildMirrors(official, c.StringSlice("preference"), standby)

	checker := NewFileChecker(mirrors)

	timeout := time.Second * time.Duration(c.Int("timeout"))
	choosedServer, results := checker.Check(filename, timeout)

	signal.Ignore(syscall.SIGPIPE, syscall.SIGIO)
	fmt.Println(choosedServer)

	err = utils.ReportChoosedServer(official, filename, choosedServer)
	if err != nil {
		fmt.Fprintln(os.Stderr, "W:", err)
	}

	err = db.Record(choosedServer, <-results)
	if err != nil {
		fmt.Fprintln(os.Stderr, "W:", err)
	}
}

func SubmainMirrorSynProgress(c *cli.Context) {
	indexName := c.String("index")
	exportPath := c.String("export")
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

	infos := DetectServer(n, indexName, official, mlist)
	ShowMirrorInfos(infos)

	f, err := os.Create(exportPath)
	if err != nil {
		fmt.Println("E:", err)
		return
	}
	err = SaveMirrorInfos(infos, f)
	if err != nil {
		fmt.Println("E:", err)
	}
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
