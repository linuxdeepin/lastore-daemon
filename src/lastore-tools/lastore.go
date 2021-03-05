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
	"fmt"
	"os"
	"strings"
	"time"

	"internal/dstore"
	"internal/system"

	"github.com/codegangsta/cli"
	"github.com/godbus/dbus"
	lastore "github.com/linuxdeepin/go-dbus-factory/com.deepin.lastore"
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
// 部分 install 和 remove 命令不能直接执行，需要把本程序的路径（一般是 /usr/bin/lastore-tools）加入配置文件
//（一般是/var/lib/lastore/config.json）中的 AllowInstallRemovePkgExecPaths 列表中。
func MainTester(c *cli.Context) {
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
		cli.ShowCommandHelp(c, "test")
	}
	if err != nil {
		fmt.Println("E:", err)
		os.Exit(-1)
	}
}

func LastoreUpdate() error {
	m := getLastore()
	fmt.Println("Connected lastore-daemon..")

	fmt.Println("Try updating source")
	j, err := m.UpdateSource(0)
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}

func LastoreRemove(p string) error {
	m := getLastore()
	fmt.Println("Connected lastore-daemon..")

	fmt.Println("Try removing", p)
	j, err := m.RemovePackage(0, "RemoveForTesing "+p, p)
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}

func LastoreInstall(p string) error {
	m := getLastore()
	fmt.Println("Connected lastore-daemon..")

	fmt.Println("Try installing", p)
	j, err := m.InstallPackage(0, "InstallForTesing "+p, p)
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}

func LastorePrepareUpgrade() error {
	m := getLastore()
	fmt.Println("Connected lastore-daemon..")

	j, err := m.PrepareDistUpgrade(0)
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
	m := getLastore()
	fmt.Println("Connected lastore-daemon..")

	fmt.Println("Try updating /var/lib/apt/lists .....")
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

func getLastore() *lastore.Lastore {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		panic(err)
	}

	return lastore.NewLastore(sysBus)
}

func showLine(j *lastore.Job) string {
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

	for l := showLine(j); status != ""; {
		t := showLine(j)
		if t != l {
			l = t
			fmt.Println(t)
		}
		switch system.Status(status) {
		case system.ReadyStatus, system.RunningStatus:
		case system.PausedStatus:
			return fmt.Errorf("job be paused")
		case system.SucceedStatus:
			fmt.Println("succeeful finished")
			return nil
		case system.FailedStatus:
			return fmt.Errorf("job %v failed %v", p, j)
		}

		time.Sleep(time.Millisecond * 50)
		status, _ = j.Status().Get(0)
	}
	return err
}
