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

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/codegangsta/cli"
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
			Name: "stats",
			Usage: `show the history of serving
     ✓ and ★ indicate the candidates in next mirror selecting.
     But ★ also indicate the mirror was unworkable in preview detecting.`,
			Action: func(c *cli.Context) {
				fmt.Println("Didn't support at this version")
			},
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

func ShowBest(url string) {
	fmt.Println(url)
}
func ShowBestOnError(url string, err error) {
	ShowBest(url)
	fmt.Printf("E:%v\n", err)
	os.Exit(-1)
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
	if strings.HasPrefix(p, "http://") ||
		strings.HasPrefix(p, "https://") {
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
