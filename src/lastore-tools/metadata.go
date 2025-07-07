// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"
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
			Value: "",
			Usage: "the remote to fetch metadata",
		},
	},
}

// MainMetadata 目前 metadata 功能被废弃
func MainMetadata(c *cli.Context) error {
	remote := c.String("remote")
	repo := c.String("local")
	checkout := c.String("checkout")

	tree, err := utils.NewOSTree(repo, remote)
	if err != nil {
		fmt.Println("NewOSTree:", err)
		return err
	}

	updateFlag := c.Bool("update")
	if updateFlag || !tree.HasBranch("origin:lastore") {
		_, _ = fmt.Fprintf(os.Stderr, "Try updating from %q to %q\n", remote, repo)
		err = tree.Pull("lastore")
		if err != nil {
			fmt.Println("pullRepo:", err)
			return err
		}
		err = tree.Checkout("lastore", checkout, false)
		if err != nil {
			fmt.Println("checkoutRepo:", err)
			return err
		}
	}

	if c.Bool("list") {
		c, err := tree.List("lastore", "/")
		fmt.Println(c, err)
		return err
	}

	for _, id := range c.Args() {
		c, err := tree.Cat("lastore", id+"/meta/manifest.json")
		if err != nil {
			fmt.Println("EC:", err)
			continue
		}
		fmt.Println(c)
	}
	return nil
}
