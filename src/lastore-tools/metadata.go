package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"internal/utils"
	"os"
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
			Value: "/lastore/metadata",
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
	if updateFlag || !tree.HasBranch("origin:master") {
		fmt.Fprintf(os.Stderr, "Try updating from %q to %q\n", remote, repo)
		err = tree.Pull("master")
		if err != nil {
			fmt.Println("pullRepo:", err)
			return
		}
		err = tree.Checkout("master", checkout, false)
		if err != nil {
			fmt.Println("checkoutRepo:", err)
			return
		}
	}

	if c.Bool("list") {
		c, err := tree.List("master", "/")
		fmt.Println(c, err)
		return
	}

	for _, id := range c.Args() {
		c, err := tree.Cat("master", id+"/meta/manifest.json")
		if err != nil {
			fmt.Printf("EC:", err)
			continue
		}
		fmt.Println(c)
	}

}
