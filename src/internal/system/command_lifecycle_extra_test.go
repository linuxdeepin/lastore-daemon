// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCmdSet implements CommandSet for tests and records removed job ids.
type fakeCmdSet struct {
	mu      sync.Mutex
	removed []string
}

func (f *fakeCmdSet) AddCMD(_ *Command) {}
func (f *fakeCmdSet) RemoveCMD(id string) {
	f.mu.Lock()
	f.removed = append(f.removed, id)
	f.mu.Unlock()
}
func (f *fakeCmdSet) FindCMD(_ string) *Command { return nil }
func (f *fakeCmdSet) removedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.removed...)
}
func (f *fakeCmdSet) hasRemoved(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.removed {
		if r == id {
			return true
		}
	}
	return false
}

// recordingIndicator captures every JobProgressInfo and signals the final
// status of the job on a buffered channel so tests can wait for completion.
type recordingIndicator struct {
	mu    sync.Mutex
	infos []JobProgressInfo
	final chan Status
}

func newRecordingIndicator() *recordingIndicator {
	return &recordingIndicator{final: make(chan Status, 4)}
}

func (r *recordingIndicator) call(info JobProgressInfo) {
	r.mu.Lock()
	r.infos = append(r.infos, info)
	r.mu.Unlock()
	switch info.Status {
	case SucceedStatus, FailedStatus, PausedStatus:
		select {
		case r.final <- info.Status:
		default:
		}
	}
}

func (r *recordingIndicator) wait(t *testing.T) Status {
	t.Helper()
	select {
	case s := <-r.final:
		return s
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for job final status")
		return ""
	}
}

func (r *recordingIndicator) infosSnapshot() []JobProgressInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]JobProgressInfo(nil), r.infos...)
}

func newEchoCommand(cmdSet CommandSet, ind Indicator) *Command {
	return &Command{
		JobId:             "job-1",
		Cancelable:        true,
		CmdSet:            cmdSet,
		Cmd:               exec.Command("echo", "hello"),
		Indicator:         ind,
		DeliveryIndicator: func(info JobDeliveryDownloadInfo) {},
		ParseProgressInfo: func(id, line string) (JobProgressInfo, error) {
			return JobProgressInfo{JobId: id, Status: RunningStatus, Progress: 0.5}, nil
		},
		ParseJobError: func(stdErr, stdOut string) *JobError { return nil },
		ParseDeliveryDownloadInfo: func(id, line string) (JobDeliveryDownloadInfo, error) {
			return JobDeliveryDownloadInfo{JobId: id}, nil
		},
	}
}

func TestCommandStartSuccessExtra(t *testing.T) {
	cmdSet := &fakeCmdSet{}
	rec := newRecordingIndicator()
	c := newEchoCommand(cmdSet, rec.call)

	require.NoError(t, c.Start())
	assert.Equal(t, SucceedStatus, rec.wait(t))
	assert.Equal(t, ExitSuccess, c.ExitCode)
	assert.True(t, cmdSet.hasRemoved("job-1"))

	// The first indicator is the "running" log entry (OnlyLog, no JobId).
	infos := rec.infosSnapshot()
	require.NotEmpty(t, infos)
	assert.True(t, infos[0].OnlyLog)
	assert.Contains(t, infos[0].OriginalLog, "job-1")
}

func TestCommandStartFailureExtra(t *testing.T) {
	cmdSet := &fakeCmdSet{}
	rec := newRecordingIndicator()
	c := newEchoCommand(cmdSet, rec.call)
	c.Cmd = exec.Command("sh", "-c", "echo 'boom' >&2; exit 1")
	c.ParseJobError = func(stdErr, stdOut string) *JobError {
		return &JobError{ErrType: ErrorDpkgError, ErrDetail: "boom"}
	}

	require.NoError(t, c.Start())
	assert.Equal(t, FailedStatus, rec.wait(t))
	assert.Equal(t, ExitFailure, c.ExitCode)
	assert.True(t, cmdSet.hasRemoved("job-1"))

	found := false
	for _, info := range rec.infosSnapshot() {
		if info.Status == FailedStatus && info.Error != nil {
			found = true
			assert.Equal(t, ErrorDpkgError, info.Error.ErrType)
		}
	}
	assert.True(t, found, "expected a FailedStatus indicator carrying the JobError")
}

func TestCommandWaitSuccessExitCodeExtra(t *testing.T) {
	cmdSet := &fakeCmdSet{}
	rec := newRecordingIndicator()
	c := newEchoCommand(cmdSet, rec.call)
	require.NoError(t, c.Start())
	rec.wait(t)

	// ExitCode is set to ExitSuccess after the command runs successfully.
	assert.Equal(t, ExitSuccess, c.ExitCode)
}

func TestCommandIndicateFailedExtra(t *testing.T) {
	cmdSet := &fakeCmdSet{}
	rec := newRecordingIndicator()
	c := newEchoCommand(cmdSet, rec.call)
	c.JobId = "job-failed"

	c.IndicateFailed(ErrorFetchFailed, "network down", true)
	assert.True(t, cmdSet.hasRemoved("job-failed"))

	infos := rec.infosSnapshot()
	require.NotEmpty(t, infos)
	last := infos[len(infos)-1]
	assert.Equal(t, FailedStatus, last.Status)
	assert.Equal(t, -1.0, last.Progress)
	assert.True(t, last.FatalError)
	require.NotNil(t, last.Error)
	assert.Equal(t, ErrorFetchFailed, last.Error.ErrType)
	assert.Equal(t, "network down", last.Error.ErrDetail)
}

func TestCommandUpdateProgressExtra(t *testing.T) {
	pr, pw, err := os.Pipe()
	require.NoError(t, err)
	defer pr.Close()

	var mu sync.Mutex
	var infos []JobProgressInfo
	c := &Command{
		JobId: "job-1",
		pipe:  pr,
		ParseProgressInfo: func(id, line string) (JobProgressInfo, error) {
			return JobProgressInfo{JobId: id, Status: RunningStatus, Progress: 0.5, Cancelable: true}, nil
		},
		Indicator: func(info JobProgressInfo) {
			mu.Lock()
			infos = append(infos, info)
			mu.Unlock()
		},
	}

	done := make(chan struct{})
	go func() {
		c.updateProgress()
		close(done)
	}()

	_, err = pw.Write([]byte("line1\nline2\n"))
	require.NoError(t, err)
	require.NoError(t, pw.Close())
	<-done

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, infos, 2)
	assert.True(t, c.Cancelable)
}

func TestCommandUpdateProgressParseErrorExtra(t *testing.T) {
	pr, pw, err := os.Pipe()
	require.NoError(t, err)
	defer pr.Close()

	var mu sync.Mutex
	var infos []JobProgressInfo
	c := &Command{
		JobId: "job-1",
		pipe:  pr,
		ParseProgressInfo: func(id, line string) (JobProgressInfo, error) {
			return JobProgressInfo{}, assert.AnError
		},
		Indicator: func(info JobProgressInfo) {
			mu.Lock()
			infos = append(infos, info)
			mu.Unlock()
		},
	}

	done := make(chan struct{})
	go func() {
		c.updateProgress()
		close(done)
	}()

	_, err = pw.Write([]byte("bad\n"))
	require.NoError(t, err)
	require.NoError(t, pw.Close())
	<-done

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, infos, 1)
	assert.True(t, infos[0].OnlyLog)
}

func TestCommandUpdateStderrExtra(t *testing.T) {
	pr, pw, err := os.Pipe()
	require.NoError(t, err)
	defer pr.Close()

	var mu sync.Mutex
	var infos []JobDeliveryDownloadInfo
	c := &Command{
		JobId:   "job-1",
		stderrR: pr,
		ParseDeliveryDownloadInfo: func(id, line string) (JobDeliveryDownloadInfo, error) {
			return JobDeliveryDownloadInfo{JobId: id, FileName: "a.deb"}, nil
		},
		DeliveryIndicator: func(info JobDeliveryDownloadInfo) {
			mu.Lock()
			infos = append(infos, info)
			mu.Unlock()
		},
	}

	done := make(chan struct{})
	go func() {
		c.updateStderr()
		close(done)
	}()

	_, err = pw.Write([]byte("102 Status line\nordinary line\n"))
	require.NoError(t, err)
	require.NoError(t, pw.Close())
	<-done

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, infos, 1)
	assert.Equal(t, "a.deb", infos[0].FileName)
}

// newAtExitCommand builds a Command whose pipes and Cmd are already started so
// that atExit can be exercised directly for a given exit code.
func newAtExitCommand(t *testing.T, exitCode int, atExitFn func() bool, ind Indicator) *Command {
	t.Helper()
	pr, _, err := os.Pipe()
	require.NoError(t, err)
	sr, _, err := os.Pipe()
	require.NoError(t, err)
	sw, _, err := os.Pipe()
	require.NoError(t, err)

	return &Command{
		JobId:         "job-1",
		CmdSet:        &fakeCmdSet{},
		Cmd:           exec.Command("true"),
		ExitCode:      exitCode,
		pipe:          pr,
		stderrR:       sr,
		stderrW:       sw,
		Indicator:     ind,
		AtExitFn:      atExitFn,
		ParseJobError: func(stdErr, stdOut string) *JobError { return nil },
	}
}

// newClosedFile returns an already-closed *os.File so that closing it again
// produces an error, exercising atExit's close-error branches.
func newClosedFile(t *testing.T) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, r.Close())
	return r
}

func TestCommandAtExitPausedExtra(t *testing.T) {
	rec := newRecordingIndicator()
	c := newAtExitCommand(t, ExitPause, nil, rec.call)
	c.atExit()
	assert.Equal(t, PausedStatus, <-rec.final)
}

func TestCommandAtExitAtExitFnReturnsTrueExtra(t *testing.T) {
	rec := newRecordingIndicator()
	called := false
	c := newAtExitCommand(t, ExitSuccess, func() bool { called = true; return true }, rec.call)
	c.atExit()
	assert.True(t, called)
	// AtExitFn returning true short-circuits: no final status is emitted.
	select {
	case s := <-rec.final:
		t.Fatalf("unexpected final status %q", s)
	default:
	}
}

func TestCommandStartExecError(t *testing.T) {
	cmdSet := &fakeCmdSet{}
	rec := newRecordingIndicator()
	c := newEchoCommand(cmdSet, rec.call)
	c.Cmd = exec.Command("/nonexistent/binary/definitely/missing")

	err := c.Start()
	assert.Error(t, err)
	// The process never started, so no final status is emitted.
	select {
	case s := <-rec.final:
		t.Fatalf("unexpected final status %q", s)
	default:
	}
}

func TestCommandAtExitUnknownStatus(t *testing.T) {
	rec := newRecordingIndicator()
	c := newAtExitCommand(t, 99, nil, rec.call)
	c.atExit()

	infos := rec.infosSnapshot()
	require.NotEmpty(t, infos)
	assert.Contains(t, infos[0].OriginalLog, "UNKNOWN")

	// An unknown exit code matches no final-status case.
	select {
	case s := <-rec.final:
		t.Fatalf("unexpected final status %q", s)
	default:
	}
}

func TestCommandAtExitFailureNoParseError(t *testing.T) {
	rec := newRecordingIndicator()
	c := newAtExitCommand(t, ExitFailure, nil, rec.call)
	c.atExit()

	// ParseJobError returns nil, so atExit falls back to SucceedStatus.
	assert.Equal(t, SucceedStatus, <-rec.final)
}

func TestCommandAtExitCloseErrors(t *testing.T) {
	rec := newRecordingIndicator()
	c := &Command{
		JobId:         "job-1",
		CmdSet:        &fakeCmdSet{},
		Cmd:           exec.Command("true"),
		ExitCode:      ExitSuccess,
		pipe:          newClosedFile(t),
		stderrR:       newClosedFile(t),
		stderrW:       newClosedFile(t),
		Indicator:     rec.call,
		ParseJobError: func(stdErr, stdOut string) *JobError { return nil },
	}
	c.atExit()
	assert.Equal(t, SucceedStatus, <-rec.final)
}

func TestCommandUpdateStderrParseError(t *testing.T) {
	pr, pw, err := os.Pipe()
	require.NoError(t, err)
	defer pr.Close()

	var mu sync.Mutex
	parsed := 0
	delivered := 0
	c := &Command{
		JobId:   "job-1",
		stderrR: pr,
		ParseDeliveryDownloadInfo: func(id, line string) (JobDeliveryDownloadInfo, error) {
			mu.Lock()
			parsed++
			mu.Unlock()
			return JobDeliveryDownloadInfo{}, assert.AnError
		},
		DeliveryIndicator: func(info JobDeliveryDownloadInfo) {
			mu.Lock()
			delivered++
			mu.Unlock()
		},
	}

	done := make(chan struct{})
	go func() {
		c.updateStderr()
		close(done)
	}()

	_, err = pw.Write([]byte("102 Status line\n"))
	require.NoError(t, err)
	require.NoError(t, pw.Close())
	<-done

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, parsed)
	assert.Equal(t, 0, delivered)
}
