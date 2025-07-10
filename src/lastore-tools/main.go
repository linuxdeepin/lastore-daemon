// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/mirrors"
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"

	"github.com/codegangsta/cli"
	"github.com/linuxdeepin/go-lib/log"
)

var CMDUpdater = cli.Command{
	Name:   "update",
	Usage:  "Update appstore information from server",
	Action: MainUpdater,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "job,j",
			Value: "",
			Usage: "categories|applications|xcategories|desktop|update_infos|mirrors|update-monitor",
		},
		cli.StringFlag{
			Name:  "repo,r",
			Value: "desktop",
			Usage: "the repository type",
		},
		cli.StringFlag{
			Name:  "output,o",
			Value: "/dev/stdout",
			Usage: "the file to write",
		},
		cli.StringFlag{
			Name:  "mirrors-url",
			Value: "",
			Usage: "",
		},
	},
}

var logger = log.NewLogger("lastore/lastore-tools")

// MainUpdater 处理 update 子命令。
// 在文件 var/lib/lastore/scripts/update_metadata_info 中被调用。
func MainUpdater(c *cli.Context) (err error) {
	// 输出文件
	fpath := c.String("output")
	job := c.String("job")
	repo := c.String("repo")

	switch job {
	case "categories":
		err = GenerateCategory(repo, fpath)
	case "applications":
		err = GenerateApplications(repo, fpath)
	case "xcategories":
		var all map[string]string
		if all, err = BuildCategories(); err == nil {
			err = writeData(fpath, all)
		}
	case "desktop":
		if fpath == "" {
			err = errors.New("which directory to save  desktop index files?")
			break
		}
		err = GenerateDesktopIndexes(fpath)
	case "update_infos":
		_ = GenerateUpdateInfos(fpath)
	case "mirrors":
		err = mirrors.GenerateMirrors(repo, fpath)
	case "unpublished-mirrors":
		url := c.String("mirrors-url")
		err = mirrors.GenerateUnpublishedMirrors(url, fpath)
	case "update-monitor":
		err = UpdateMonitor()
	default:
		_ = cli.ShowCommandHelp(c, "update")
	}
	if err != nil {
		fmt.Println("E:", err)
		os.Exit(-1)
	}
	return err
}

func main() {
	// 清除语言相关环境变量
	_ = utils.UnsetEnv("LC_ALL")
	_ = utils.UnsetEnv("LANGUAGE")
	_ = utils.UnsetEnv("LC_MESSAGES")
	_ = utils.UnsetEnv("LANG")

	http.DefaultClient.Timeout = 60 * time.Second
	app := cli.NewApp()
	app.Name = "lastore-tools"
	app.Usage = "help building dstore system."
	app.Version = "0.9.18"
	// 定义全局选项
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug,d",
			Usage: "show verbose message",
		},
		cli.StringFlag{
			Name:  "dstoreapi",
			Usage: "the dstore api server url. There has many jobs would use the to fetch data",
			Value: "",
		},
	}
	app.Commands = []cli.Command{CMDUpdater, CMDTester, CMDSmartMirror, CMDMetadata, CMDQueryDesktop, CMDCheckPolicy, CMDPostUpgrade}

	err := app.Run(os.Args)
	if err != nil {
		logger.Fatal(err)
	}
}
