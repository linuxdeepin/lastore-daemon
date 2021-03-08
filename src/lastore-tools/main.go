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
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/codegangsta/cli"
	"pkg.deepin.io/lib/utils"

	"internal/mirrors"
)

var CMDUpdater = cli.Command{
	Name:   "update",
	Usage:  "Update appstore information from server",
	Action: MainUpdater,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "job,j",
			Value: "",
			Usage: "categories|applications|xcategories|desktop|update_infos|mirrors",
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

// MainUpdater 处理 update 子命令。
// 在文件 var/lib/lastore/scripts/update_metadata_info 中被调用。
func MainUpdater(c *cli.Context) {
	var err error

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
	default:
		cli.ShowCommandHelp(c, "update")
		return
	}
	if err != nil {
		fmt.Println("E:", err)
		os.Exit(-1)
	}
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
			Value: "http://api.appstore.deepin.org",
		},
	}
	app.Commands = []cli.Command{CMDUpdater, CMDTester, CMDSmartMirror, CMDMetadata, CMDQueryDesktop}

	app.RunAndExitOnError()
}
