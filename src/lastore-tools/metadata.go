/*
 * Copyright (C) 2017 ~ 2017 Deepin Technology Co., Ltd.
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
	"fmt"
	"internal/utils"
	"os"

	"github.com/codegangsta/cli"
)

var CMDMetadata = cli.Command{
	Name:   "metadata",
	Usage:  `package id`,
	Action: MainMetadata,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "update,u",
			Usage: "update cache message",
		},
		cli.BoolFlag{
			Name:  "list,l",
			Usage: "list metadata and quit",
		},
		cli.StringFlag{
			Name:  "local",
			Value: "/var/lib/lastore/tree",
			Usage: "the local ostree repo",
		},
		cli.StringFlag{
			Name:  "checkout,c",
			Value: "/lastore",
			Usage: "the directory to checkout the metadata",
		},
		cli.StringFlag{
			Name:  "remote",
			Value: "http://cdn.packages.deepin.com/deepin/tree/lastore",
			Usage: "the remote to fetch metadata",
		},
	},
}

func MainMetadata(c *cli.Context) {
	remote := c.String("remote")
	repo := c.String("local")
	checkout := c.String("checkout")

	tree, err := utils.NewOSTree(repo, remote)
	if err != nil {
		fmt.Println("NewOSTree:", err)
		return
	}

	updateFlag := c.Bool("update")
	if updateFlag || !tree.HasBranch("origin:lastore") {
		fmt.Fprintf(os.Stderr, "Try updating from %q to %q\n", remote, repo)
		err = tree.Pull("lastore")
		if err != nil {
			fmt.Println("pullRepo:", err)
			return
		}
		err = tree.Checkout("lastore", checkout, false)
		if err != nil {
			fmt.Println("checkoutRepo:", err)
			return
		}
	}

	if c.Bool("list") {
		c, err := tree.List("lastore", "/")
		fmt.Println(c, err)
		return
	}

	for _, id := range c.Args() {
		c, err := tree.Cat("lastore", id+"/meta/manifest.json")
		if err != nil {
			fmt.Println("EC:", err)
			continue
		}
		fmt.Println(c)
	}

}
