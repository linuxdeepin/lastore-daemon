// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/linuxdeepin/lastore-daemon/src/internal/querydesktop"

	"github.com/codegangsta/cli"
)

// TODO remove
var CMDQueryDesktop = cli.Command{
	Name:  "querydesktop",
	Usage: `pkgname`,
	Action: func(ctx *cli.Context) error {
		querydesktop.InitDB()
		if ctx.NArg() != 1 {
			_ = cli.ShowAppHelp(ctx)
			return nil
		}
		fmt.Println(querydesktop.QueryDesktopFile(ctx.Args()[0]))
		return nil
	},
}
