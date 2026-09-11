// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddCMD(t *testing.T) {
	p := &APTSystem{CmdSet: make(map[string]*system.Command)}
	cmd := &system.Command{JobId: "job1", ExitCode: 0}
	p.AddCMD(cmd)
	assert.Equal(t, 1, len(p.CmdSet))
	assert.Same(t, cmd, p.CmdSet["job1"])

	cmd2 := &system.Command{JobId: "job1", ExitCode: 1}
	p.AddCMD(cmd2)
	assert.Equal(t, 1, len(p.CmdSet), "duplicate AddCMD should not overwrite")
	assert.Same(t, cmd, p.CmdSet["job1"])
}

func TestRemoveCMD(t *testing.T) {
	p := &APTSystem{CmdSet: make(map[string]*system.Command)}
	cmd := &system.Command{JobId: "job1", ExitCode: 0}
	p.AddCMD(cmd)
	p.RemoveCMD("job1")
	assert.Equal(t, 0, len(p.CmdSet))

	p.RemoveCMD("nonexistent")
	assert.Equal(t, 0, len(p.CmdSet))
}

func TestFindCMD(t *testing.T) {
	p := &APTSystem{CmdSet: make(map[string]*system.Command)}
	cmd := &system.Command{JobId: "job1", ExitCode: 0}
	p.AddCMD(cmd)

	found := p.FindCMD("job1")
	assert.Same(t, cmd, found)

	notFound := p.FindCMD("nonexistent")
	assert.Nil(t, notFound)
}

func TestCreateCommandLine(t *testing.T) {
	tests := []struct {
		name        string
		cmdType     string
		cmdArgs     []string
		wantPathSub string
		wantContain []string
	}{
		{
			name:        "install",
			cmdType:     system.InstallJobType,
			cmdArgs:     []string{"vim"},
			wantPathSub: "apt-get",
			wantContain: []string{"install", "vim"},
		},
		{
			name:        "remove",
			cmdType:     system.RemoveJobType,
			cmdArgs:     []string{"vim"},
			wantPathSub: "apt-get",
			wantContain: []string{"autoremove", "vim"},
		},
		{
			name:        "dist_upgrade",
			cmdType:     system.DistUpgradeJobType,
			cmdArgs:     []string{},
			wantPathSub: "apt-get",
			wantContain: []string{"dist-upgrade", "--allow-downgrades"},
		},
		{
			name:        "prepare_dist_upgrade",
			cmdType:     system.PrepareDistUpgradeJobType,
			cmdArgs:     []string{},
			wantPathSub: "apt-get",
			wantContain: []string{"dist-upgrade", "-d", "-m"},
		},
		{
			name:        "download",
			cmdType:     system.DownloadJobType,
			cmdArgs:     []string{"vim"},
			wantPathSub: "apt-get",
			wantContain: []string{"install", "-d", "-m"},
		},
		{
			name:        "update_source",
			cmdType:     system.UpdateSourceJobType,
			cmdArgs:     []string{},
			wantPathSub: "apt-get",
			wantContain: []string{"update", "--fix-missing"},
		},
		{
			name:        "clean",
			cmdType:     system.CleanJobType,
			cmdArgs:     []string{},
			wantPathSub: "lastore-apt-clean",
			wantContain: []string{},
		},
		{
			name:        "backup",
			cmdType:     system.BackupJobType,
			cmdArgs:     []string{},
			wantPathSub: "deepin-immutable-ctl",
			wantContain: []string{"admin", "deploy", "--backup"},
		},
		{
			name:        "incremental_download",
			cmdType:     system.IncrementalDownloadJobType,
			cmdArgs:     []string{"vim"},
			wantPathSub: "deepin-immutable-ctl",
			wantContain: []string{"upgrade", "--download-only", "vim"},
		},
		{
			name:        "incremental_update",
			cmdType:     system.IncrementalUpdateJobType,
			cmdArgs:     []string{"vim"},
			wantPathSub: "deepin-immutable-ctl",
			wantContain: []string{"upgrade", "vim"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := createCommandLine(tt.cmdType, tt.cmdArgs)
			assert.NotNil(t, cmd)
			assert.Contains(t, cmd.Path, tt.wantPathSub)
			for _, s := range tt.wantContain {
				assert.Contains(t, cmd.Args, s)
			}
		})
	}
}

func TestCreateCommandLineFixError(t *testing.T) {
	cmd := createCommandLine(system.FixErrorJobType, []string{string(system.ErrorDpkgInterrupted)})
	assert.NotNil(t, cmd)
	assert.Contains(t, cmd.Args, "install")

	cmd2 := createCommandLine(system.FixErrorJobType, []string{string(system.ErrorDependenciesBroken)})
	assert.NotNil(t, cmd2)
	assert.Contains(t, cmd2.Args, "install")
}

func TestCreateCommandLineFixErrorPanic(t *testing.T) {
	assert.Panics(t, func() {
		createCommandLine(system.FixErrorJobType, []string{"invalid_error_type"})
	})
}

func TestNewAPTCommand(t *testing.T) {
	p := &APTSystem{CmdSet: make(map[string]*system.Command)}
	cmd := newAPTCommand(p, "job-1", system.InstallJobType, nil, nil, []string{"vim"})

	require.NotNil(t, cmd)
	assert.Equal(t, "job-1", cmd.JobId)
	assert.True(t, cmd.Cancelable)
	assert.Len(t, p.CmdSet, 1)
	assert.Same(t, cmd, p.CmdSet["job-1"])

	// command line is an install of vim via apt-get
	assert.Contains(t, cmd.Cmd.Path, "apt-get")
	assert.Contains(t, cmd.Cmd.Args, "install")
	assert.Contains(t, cmd.Cmd.Args, "vim")

	// process group is set so Abort can kill the whole tree
	require.NotNil(t, cmd.Cmd.SysProcAttr)
	assert.True(t, cmd.Cmd.SysProcAttr.Setpgid)

	// parse callbacks are wired to the apt-specific parsers
	assert.NotNil(t, cmd.ParseJobError)
	assert.NotNil(t, cmd.ParseProgressInfo)
	assert.NotNil(t, cmd.ParseDeliveryDownloadInfo)

	// stdout is captured into the command's own buffer
	assert.NotNil(t, cmd.Cmd.Stdout)
}
