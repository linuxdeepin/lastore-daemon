// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/dstore"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

	"github.com/codegangsta/cli"
	"github.com/godbus/dbus/v5"
	lastore "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.lastore1"
)

var CMDTester = cli.Command{
	Name: "test",
	Usage: `Use lastore-daemon to run jobs
    search will search apps from dstore. It will list all apps
    if there hasn't any input.

    install/remove will execute the command with the input
    package name.

    upgrade will first update source and then upgrade packages
    if there has any one.
`,
	Action: MainTester,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "job,j",
			Value: "",
			Usage: "install|remove|upgrade|prepare_upgrade|search|update",
		},
	},
}

// MainTester 处理 test 子命令。
// 其中 install 和 remove 命令不能直接执行，需要把本程序的路径（一般是 /usr/bin/lastore-tools）加入配置文件
//（一般是/var/lib/lastore/config.json）中的 AllowInstallRemovePkgExecPaths 列表中。
func MainTester(c *cli.Context) error {
	var err error
	switch c.String("job") {
	case "install":
		err = LastoreInstall(c.Args().First())
	case "remove":
		err = LastoreRemove(c.Args().First())
	case "upgrade":
		err = LastoreUpgrade()
	case "update":
		err = LastoreUpdate()
	case "search":
		err = LastoreSearch("", c.Args().First(), c.GlobalBool("debug"))
	case "prepare_upgrade":
		err = LastorePrepareUpgrade()
	default:
		_ = cli.ShowCommandHelp(c, "test")
	}
	if err != nil {
		fmt.Println("E:", err)
		os.Exit(-1)
	}
	return err
}

func LastoreUpdate() error {
	l := getLastore()
	fmt.Println("Connected lastore-daemon..")

	fmt.Println("Try updating source")
	j, err := l.Manager().UpdateSource(0)
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}

func LastoreRemove(p string) error {
	l := getLastore()
	fmt.Println("Connected lastore-daemon..")

	fmt.Println("Try removing", p)
	j, err := l.Manager().RemovePackage(0, "RemoveForTesting "+p, p)
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}

func LastoreInstall(p string) error {
	l := getLastore()
	fmt.Println("Connected lastore-daemon..")

	fmt.Println("Try installing", p)
	j, err := l.Manager().InstallPackage(0, "InstallForTesting "+p, p)
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}

func LastorePrepareUpgrade() error {
	l := getLastore()
	fmt.Println("Connected lastore-daemon..")

	j, err := l.Manager().PrepareDistUpgrade(0)
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}

func LastoreSearch(server string, p string, debug bool) error {
	store := dstore.NewStore()
	pkgInfos, err := store.GetPackageApplication("/var/lib/lastore/applications.json")
	if err != nil {
		return err
	}
	for _, info := range pkgInfos {
		if p == "" || strings.Contains(info.PackageName, p) {
			if debug {
				fmt.Printf("-%s %v\n", info.PackageName, info)
			} else {
				fmt.Println(info.PackageName)
			}
		}
	}
	return nil
}

func LastoreUpgrade() error {
	l := getLastore()
	fmt.Println("Connected lastore-daemon..")

	m := l.Manager()

	fmt.Println("Try updating /var/lib/lastore/lists .....")
	j, err := m.UpdateSource(0)

	if err != nil {
		fmt.Printf("Created Job: %v failed\n", err)
		return err
	}
	if err = waitJob(j); err != nil {
		return err
	}
	fmt.Printf("Created Job: %q successful\n", j)

	fmt.Println()

	list, err := m.UpgradableApps().Get(0)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("There hasn't any packages need be upgrade.")
		return nil
	}

	fmt.Printf("Try upgrading %v\n", list)
	j, err = m.DistUpgrade(0)
	if err != nil {
		return err
	}
	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}

func getLastore() lastore.Lastore {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		panic(err)
	}

	return lastore.NewLastore(sysBus)
}

func showLine(j lastore.Job) string {
	id, _ := j.Id().Get(0)
	type0, _ := j.Type().Get(0)
	status, _ := j.Status().Get(0)
	progress, _ := j.Progress().Get(0)
	description, _ := j.Description().Get(0)

	return fmt.Sprintf("id:%v(%v)\tProgress:%v:%v%%\tDesc:%q",
		id, type0, status, progress*100, description)
}

func waitJob(p dbus.ObjectPath) error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	j, err := lastore.NewJob(sysBus, p)
	if err != nil {
		return err
	}

	status, _ := j.Status().Get(0)
	if status != "" {
		fmt.Println(showLine(j))
	}

	// 每隔 50ms 查询一次状态
	for l := showLine(j); status != ""; {
		t := showLine(j)
		// 状态行发生改变才打印出来
		if t != l {
			l = t
			fmt.Println(t)
		}
		switch system.Status(status) {
		case system.ReadyStatus, system.RunningStatus:
		case system.PausedStatus:
			return fmt.Errorf("job be paused")
		case system.SucceedStatus:
			fmt.Println("successful finished")
			return nil
		case system.FailedStatus:
			return fmt.Errorf("job %v failed %v", p, j)
		}

		time.Sleep(time.Millisecond * 50)
		status, _ = j.Status().Get(0)
	}
	return err
}
