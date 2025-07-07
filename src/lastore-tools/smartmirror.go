// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/internal/mirrors"

	"github.com/codegangsta/cli"
)

var CMDSmartMirror = cli.Command{
	Name:  "smartmirror",
	Usage: `query of select mirrors`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "official",
			Value: "",
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
			Usage: "silent mode, only produces necessary output",
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
			Action: func(c *cli.Context) error {
				fmt.Println("Didn't support at this version")
				return nil
			},
		},
		{
			Name:   "server_stats",
			Usage:  "report the status of all known mirrors",
			Action: SubmainMirrorSynProgress,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "index,i",
					Value: ".__GUARD__INDEX__",
					Usage: "the guard index path in official",
				},
				cli.BoolFlag{
					Name:  "list,l",
					Usage: "only list known mirrors",
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

func SubmainMirrorSynProgress(c *cli.Context) error {
	indexName := c.String("index")
	exportPath := c.String("export")
	official := c.Parent().String("official")

	onlyList := c.Bool("list")
	n := c.Parent().Int("parallel")

	// 只列出镜像源的 url 列表
	if onlyList {
		mlist, _ := getMirrorList(c.Parent().String("mirrorlist"))
		for _, m := range mlist {
			fmt.Println(m)
		}
		return nil
	}

	// 如果未指定参数，则使用所有的镜像源列表
	mlist := c.Args()
	if len(mlist) == 0 {
		mlist, _ = getMirrorList(c.Parent().String("mirrorlist"))
	}

	infos := DetectServer(n, indexName, official, mlist)

	f, err := os.Create(exportPath)
	if err != nil {
		fmt.Println("E:", err)
		return err
	}
	err = SaveMirrorInfos(infos, f)
	if err != nil {
		fmt.Println("E:", err)
	}
	return err
}

// appendSuffix 如果 r 没有后缀 suffix，则加上。
func appendSuffix(r string, suffix string) string {
	if strings.HasSuffix(r, suffix) {
		return r
	}
	return r + suffix
}

// getMirrorList 从来源 p 获取镜像源的 url 列表，p 可以是本地文件路径，或者是 http:// 或 https:// 开头的 url。
func getMirrorList(p string) ([]string, error) {
	if strings.HasPrefix(p, "http://") ||
		strings.HasPrefix(p, "https://") {
		ms, err := mirrors.LoadMirrorSources(p)
		if err != nil {
			return nil, err
		}

		var r []string
		for _, m := range ms {
			r = append(r, m.Url)
		}
		return r, nil
	}

	// 把 p 当作文件路径，解码 JSON 到 raw 中
	// #nosec G304
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

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
