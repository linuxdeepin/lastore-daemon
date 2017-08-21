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
	"github.com/codegangsta/cli"
	"os"
	"pkg.deepin.io/lib/utils"
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
	},
}

func MainUpdater(c *cli.Context) {
	var err error

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
			err = fmt.Errorf("which directory to save  desktop index files?")
		}
		err = GenerateDesktopIndexes(fpath)
	case "update_infos":
		GenerateUpdateInfos(fpath)
	case "mirrors":
		err = GenerateMirrors(repo, fpath)
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
	utils.UnsetEnv("LC_ALL")
	utils.UnsetEnv("LANGUAGE")
	utils.UnsetEnv("LC_MESSAGES")
	utils.UnsetEnv("LANG")

	app := cli.NewApp()
	app.Name = "lastore-tools"
	app.Usage = "help building dstore system."
	app.Version = "0.9.18"
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

func debugPrint(fmtStr string, args ...interface{}) {
	os.Stderr.WriteString(fmt.Sprintf(fmtStr, args...))
}
