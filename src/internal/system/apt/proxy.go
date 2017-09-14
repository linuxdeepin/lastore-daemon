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

package apt

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"internal/system"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type APTSystem struct {
	cmdSet    map[string]*aptCommand
	indicator system.Indicator
}

func New() system.System {
	p := &APTSystem{
		cmdSet: make(map[string]*aptCommand),
	}
	PrepareRunApt()
	exec.Command("/var/lib/lastore/scripts/build_safecache.sh").Run()
	return p
}

func parseProgressField(v string) (float64, error) {
	progress, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return -1, fmt.Errorf("unknown progress value: %q", v)
	}
	return progress, nil
}

func ParseProgressInfo(id, line string) (system.JobProgressInfo, error) {
	fs := strings.SplitN(line, ":", 4)
	if len(fs) != 4 {
		return system.JobProgressInfo{JobId: id}, fmt.Errorf("Invlaid Progress line:%q", line)
	}

	progress, err := parseProgressField(fs[2])
	if err != nil {
		return system.JobProgressInfo{JobId: id}, err
	}
	description := strings.TrimSpace(fs[3])

	var status system.Status
	var cancelable = true

	infoType := fs[0]

	switch infoType {
	case "dummy":
		status = system.Status(fs[1])
	case "dlstatus":
		progress = progress / 100.0
		status = system.RunningStatus
	case "pmstatus":
		progress = progress / 100.0
		status = system.RunningStatus
		cancelable = false
	case "pmerror":
		progress = -1
		status = system.FailedStatus

	default:
		//	case "pmconffile", "media-change":
		return system.JobProgressInfo{JobId: id},
			fmt.Errorf("W: unknow status:%q", line)

	}

	return system.JobProgressInfo{
		JobId:       id,
		Progress:    progress,
		Description: description,
		Status:      status,
		Cancelable:  cancelable,
	}, nil
}

func (p *APTSystem) AttachIndicator(f system.Indicator) {
	p.indicator = f
}

func (p *APTSystem) Download(jobId string, packages []string) error {
	c := newAPTCommand(p, jobId, system.DownloadJobType, p.indicator, packages)
	return c.Start()
}

func PrepareRunApt() {
	if msg, wait := checkLock("/var/lib/dpkg/lock"); !wait {
		if checkDpkgDirtyJournal() {
			tryFixDpkgDirtyStatus()
		}
	} else {
		log.Warnf("Wait 5s for unlock\n\"%s\" \n at %v\n",
			msg, time.Now())
		<-time.After(time.Second * 5)
		PrepareRunApt()
	}
}

func checkLock(p string) (string, bool) {
	cmd := exec.Command("lslocks", "-J")
	f, err := cmd.StdoutPipe()
	if err != nil {
		return "", false
	}
	cmd.Start()

	d := json.NewDecoder(f)
	var data = struct {
		Locks []map[string]string `json:"locks"`
	}{}
	d.Decode(&data)
	cmd.Wait()
	for _, line := range data.Locks {
		if line["path"] == p {
			bs, err := exec.Command("ps",
				"-p",
				line["pid"],
				"-o",
				"pid,ppid,tty,cmd",
			).Output()
			if err != nil {
				return "", false
			}
			return string(bs), true
		}
	}
	return "", false
}

// CheckDpkgDirtyJournal check if the dpkg in dirty status
// Return true if dirty. Dirty status should be fix
// by FixDpkgDirtyJournal().
// See also debsystem.cc:CheckUpdates in apt project
func checkDpkgDirtyJournal() bool {
	const updateDir = "/var/lib/dpkg/updates"
	fs, err := ioutil.ReadDir(updateDir)
	if err != nil {
		return false
	}
	for _, finfo := range fs {
		dirty := true
		for _, c := range finfo.Name() {
			if !unicode.IsDigit(rune(c)) {
				dirty = false
				break
			}
		}
		if dirty {
			return true
		}
	}
	return false
}

func tryFixDpkgDirtyStatus() {
	cmd := exec.Command("dpkg", "--force-confold", "--configure", "-a")
	buf := new(bytes.Buffer)
	cmd.Stdout = buf
	cmd.Stderr = buf
	cmd.Start()

	err := cmd.Wait()
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	log.Warn(fmt.Sprintf("Dpkg in dirty status, try fixing. %s\n", errStr))
	log.Warnf("%s\n", buf.String())
	log.Warn(fmt.Sprintf("Stage one: FixDpkg: %v\n", err))

	cmd = exec.Command("apt-get", "-f", "install", "-c", "/var/lib/lastore/apt.conf")
	cmd.Stdout = buf
	cmd.Stderr = buf
	cmd.Start()

	err = cmd.Wait()
	errStr = ""
	if err != nil {
		errStr = err.Error()
	}
	log.Warn(fmt.Sprintf("Stage two: fixing apt-get -f install. %s\n", errStr))
	log.Warnf("%s\n", buf.String())
	log.Warn(fmt.Sprintf("End of FixDpkg: %v\n", err))

}

// remove package checker
type rmPkgChecker struct {
	buf    bytes.Buffer // line buffer
	remove bool
}

// impl io.Writer
func (c *rmPkgChecker) Write(data []byte) (n int, err error) {
	if c.remove {
		return len(data), nil
	}
	for _, b := range data {
		if b != '\n' {
			err = c.buf.WriteByte(b)
			if err != nil {
				return
			}
		} else {
			// b is newline
			line := c.buf.Bytes()
			// remove package dde?
			if bytes.HasPrefix(line, []byte("Remv dde ")) {
				c.remove = true
				return len(data), nil
			}
			c.buf.Reset()
		}
		n++
	}
	return
}

func safeStart(c *aptCommand) error {
	args := c.apt.Args
	// add -s option
	args = append([]string{"-s"}, args[1:]...)
	cmd := exec.Command("apt-get", args...)
	var checker rmPkgChecker
	cmd.Stdout = &checker
	// perform apt-get action simulate
	err := cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			c.indicateFailed("apt-get simulate failed " + err.Error())
			return
		}

		// cmd run ok
		// check rm dde?
		if checker.remove {
			c.indicateFailed("remove dde")
			return
		}

		// really perform apt-get action
		err = c.Start()
		if err != nil {
			c.indicateFailed("apt-get start failed " + err.Error())
		}
	}()
	return nil
}

func (p *APTSystem) Remove(jobId string, packages []string) error {
	PrepareRunApt()
	c := newAPTCommand(p, jobId, system.RemoveJobType, p.indicator, packages)
	return safeStart(c)
}

func (p *APTSystem) Install(jobId string, packages []string) error {
	PrepareRunApt()
	c := newAPTCommand(p, jobId, system.InstallJobType, p.indicator, packages)
	return safeStart(c)
}

func (p *APTSystem) DistUpgrade(jobId string) error {
	PrepareRunApt()
	c := newAPTCommand(p, jobId, system.DistUpgradeJobType, p.indicator, nil)
	return safeStart(c)
}

func (p *APTSystem) UpdateSource(jobId string) error {
	c := newAPTCommand(p, jobId, system.UpdateSourceJobType, p.indicator, nil)
	return c.Start()
}

func (p *APTSystem) Clean(jobId string) error {
	c := newAPTCommand(p, jobId, system.CleanJobType, p.indicator, nil)
	return c.Start()
}

func (p *APTSystem) Abort(jobId string) error {
	if c := p.FindCMD(jobId); c != nil {
		return c.Abort()
	}
	return system.NotFoundError
}
