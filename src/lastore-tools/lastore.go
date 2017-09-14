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

import "dbus/com/deepin/lastore"
import "pkg.deepin.io/lib/dbus"
import "net/http"
import "encoding/json"
import "fmt"
import "time"
import "internal/system"
import "strings"
import "github.com/codegangsta/cli"
import "os"

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
		err = LastoreSearch(c.GlobalString("dstoreapi"), c.Args().First(), c.GlobalBool("debug"))
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
	j, err := m.UpdateSource()
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
	j, err := m.RemovePackage("RemoveForTesing "+p, p)
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
	j, err := m.InstallPackage("InstallForTesing "+p, p)
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

	j, err := m.PrepareDistUpgrade()
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}

func LastoreSearch(server string, p string, debug bool) error {
	resp, err := http.Get(server + "/info/all")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	apps := make(map[string]interface{})
	d := json.NewDecoder(resp.Body)
	err = d.Decode(&apps)
	if err != nil {
		return err
	}

	for app, info := range apps {
		if p == "" || strings.Contains(app, p) {
			if debug {
				fmt.Printf("-%s %v\n", app, info)
			} else {
				fmt.Println(app)
			}
		}
	}
	return nil
}

func LastoreUpgrade() error {
	m := getLastore()
	fmt.Println("Connected lastore-daemon..")

	fmt.Println("Try updating /var/lib/apt/lists .....")
	j, err := m.UpdateSource()

	if err != nil {
		fmt.Printf("Created Job: %v failed\n", err)
		return err
	}
	if err = waitJob(j); err != nil {
		return err
	}
	fmt.Printf("Created Job: %q successful\n", j)

	fmt.Println()

	list := m.UpgradableApps.Get()
	if len(list) == 0 {
		fmt.Println("There hasn't any packages need be upgrade.")
		return nil
	}

	fmt.Printf("Try upgrading %v\n", list)
	j, err = m.DistUpgrade()
	if err != nil {
		return err
	}
	fmt.Printf("Created Job: %q successful\n", j)

	return waitJob(j)
}

func getLastore() *lastore.Manager {
	m, err := lastore.NewManager("com.deepin.lastore", "/com/deepin/lastore")
	if err != nil {
		panic(err)
	}
	return m
}

func showLine(j *lastore.Job) string {
	return fmt.Sprintf("id:%v(%v)\tProgress:%v:%v%%\tDesc:%q",
		j.Id.Get(), j.Type.Get(), j.Status.Get(), j.Progress.Get()*100,
		j.Description.Get())
}

func waitJob(p dbus.ObjectPath) error {
	j, err := lastore.NewJob("com.deepin.lastore", p)
	if err != nil {
		return err
	}

	s := j.Status.Get()
	if s != "" {
		fmt.Println(showLine(j))
	}

	for l := showLine(j); s != ""; {
		t := showLine(j)
		if t != l {
			l = t
			fmt.Println(t)
		}
		switch system.Status(s) {
		case system.ReadyStatus, system.RunningStatus:
		case system.PausedStatus:
			return fmt.Errorf("Job be paused.")
		case system.SucceedStatus:
			fmt.Println("Succeeful finished.")
			return nil
		case system.FailedStatus:
			return fmt.Errorf("Job %v failed %v", p, j)
		}

		time.Sleep(time.Millisecond * 50)
		s = j.Status.Get()
	}
	return err
}
