package apt

import (
	"bufio"
	"fmt"
	"internal/system"
	"log"
	"os"
	"os/exec"
	"strconv"
)

type aptCommand struct {
	c *exec.Cmd

	reader *os.File

	indicator system.Indicator
}

func (c aptCommand) Start() {
	rr, ww, err := os.Pipe()
	defer ww.Close()
	if err != nil {
		log.Println("os.Pipe error:", err)
	}
	log.Println("Actual FD:", rr.Fd(), ww.Fd(), err)
	log.Println("Start...", c)
	c.c.ExtraFiles = append(c.c.ExtraFiles, ww)
	c.c.Stderr = os.Stderr
	c.c.Stdout = os.Stdout
	c.reader = rr

	c.c.Start()

	go c.update()
	go func() {
		err := c.c.Wait()
		ShowFd(os.Getpid())
		ww.Close()
		fmt.Println("----------------Waited..", c.c.Args, err)

		var line string
		if err != nil {
			line = "dstatus:" + system.FailedStatus + ":" + err.Error()
		} else {
			line = "dstatus:" + system.SuccessedStatus + ":successed"
		}
		info, err := ParseProgressInfo("INDICATOR WILL SET THIS", line)
		c.indicator(info)
	}()
}

func newAptCommand(fn func(system.ProgressInfo), options map[string]string, args ...string) aptCommand {
	if options == nil {
		options = make(map[string]string)
	}
	options["APT::Status-Fd"] = strconv.Itoa(3)
	for key, value := range options {
		args = append(args, "-o", key+"="+value)
	}
	args = append(args, "-y")

	c := aptCommand{
		indicator: fn,
		c:         exec.Command("/usr/bin/apt-get", args...),
	}
	c.c.Stderr = os.Stderr
	c.c.Stdout = os.Stdout
	ShowFd(os.Getpid())
	return c
}

func (c aptCommand) update() {
	b := bufio.NewReader(c.reader)
	for {
		log.Println("HH:", c.reader.Fd())
		line, err := b.ReadString('\n')
		if err != nil {
			return
		}

		info, _ := ParseProgressInfo("", line)
		c.indicator(info)
	}
}

func ShowFd(pid int) {
	return
	log.Println("PID:", pid)
	out, err := exec.Command("/bin/ls", "-lh", fmt.Sprintf("/proc/%d/fd", pid)).Output()
	log.Println(string(out), err)
	log.Println("__ENDPID__________")
}

func buildDownloadCommand(fn system.Indicator, packageId string) aptCommand {
	options := map[string]string{
		"Debug::NoLocking": "1",
		"dir::cache":       "/dev/shm/cache",
	}

	var args []string
	args = append(args, "install")
	args = append(args, "-d")
	args = append(args, packageId)
	return newAptCommand(fn, options, args...)
}

func buildInstallCommand(fn system.Indicator, packageId string) aptCommand {
	var args []string
	args = append(args, "install")
	args = append(args, packageId)
	return newAptCommand(fn, nil, args...)
}

func buildRemoveCommand(fn system.Indicator, packageId string) aptCommand {
	var args []string
	args = append(args, "remove")
	args = append(args, packageId)
	return newAptCommand(fn, nil, args...)
}

func (c aptCommand) Abort(jobId string) error {
	return system.NotImplementError
}
