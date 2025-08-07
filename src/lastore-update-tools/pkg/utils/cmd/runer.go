package runcmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
)

// run command output
func RunnerOutput(timeout int, cmd string, args ...string) (string, error) {
	env := []string{""}
	msg, out, err := execAndWait(timeout, cmd, env, args...)
	if err != nil {
		log.Debugf("run command %+v\n %s failed\n", err, out)
		return msg, fmt.Errorf("%v\n %s failed", err, out)
	}
	return msg, nil
}

// run command output
func RunnerOutputEnv(timeout int, cmd string, env []string, args ...string) (string, error) {
	msg, out, err := execAndWait(timeout, cmd, env, args...)
	if err != nil {
		log.Debugf("run command %+v\n %s failed\n", err, out)
		return msg, fmt.Errorf("%v\n %s failed", err, out)
	}
	return msg, nil
}

// run command output
func RunnerNotOutput(timeout int, cmd string, args ...string) error {

	env := []string{""}
	_, out, err := execAndWait(timeout, cmd, env, args...)
	if err != nil {
		log.Debugf("run command %+v\n %s failed\n", err, out)
		return err
	}
	return nil
}

// exec and wait for command
func execAndWait(timeout int, name string, env []string, arg ...string) (stdout, stderr string, err error) {
	log.Debugf("cmd: %s %+v %+v\n", name, arg, env)
	cmd := exec.Command(name, arg...)
	var bufStdout, bufStderr bytes.Buffer
	cmd.Stdout = &bufStdout
	cmd.Stderr = &bufStderr
	cmd.Env = os.Environ()
	if len(env) > 0 {
		cmd.Env = append(cmd.Env, env...)
	}
	err = cmd.Start()
	defer cmd.Process.Kill()
	if err != nil {
		err = fmt.Errorf("start fail: %w\n", err)
		return
	}

	// wait for process finished
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		if err = cmd.Process.Kill(); err != nil {
			err = fmt.Errorf("timeout: %w", err)
			return
		}
		<-done
		err = fmt.Errorf("time out and process was killed")
	case err = <-done:
		stdout = bufStdout.String()
		stderr = bufStderr.String()
		if err != nil {
			err = fmt.Errorf("run: %v", err)
			return
		}
	}
	return
}
