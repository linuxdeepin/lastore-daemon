package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"internal/querydesktop"
)

var CMDQueryDesktop = cli.Command{
	Name:  "querydesktop",
	Usage: `pkgname`,
	Action: func(ctx *cli.Context) {
		querydesktop.InitDB()
		if ctx.NArg() != 1 {
			cli.ShowAppHelp(ctx)
			return
		}
		fmt.Println(querydesktop.QueryDesktopFile(ctx.Args()[0]))
	},
}
