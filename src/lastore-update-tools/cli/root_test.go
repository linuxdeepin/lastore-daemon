// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"os"
	"testing"

	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

func createTestData() error {
	// /tmp/metacfg.json

	packageUrl := "xxxxs://example.com/testapp"
	tmpJson := `
{
        "PkgDebPath":"/tmp/",
        "PkgList": [
            {
                "Name": "systemd",
                "Version": "241.34-1+dde",
                "Need": "strict"
            },
            {
                "Name": "libsystemd0:amd64",
                "Version": "241.34-1+dde",
                "Need": "strict"
            }
        ],
        "CoreList": [
            {
                "Name": "systemd",
                "Version": "241.34-1+dde",
                "Need": "strict"
            },
            {
                "Name": "libsystemd0",
                "Version": "241.34-1+dde",
                "Need": "skipversion"
            },
            {
                "Name": "permission-manager",
                "Version": "0.1.124",
                "Need": "skipstate"
            }
    
        ],
        "Rules": [
            {
                "Name": "00_precheck",
                "Type": 0,
                "Command": "echo \"This is precheck\"\nexit 0\n",
                "Argv": "--ignore-warring --ignore-error"
            },
            {
                "Name": "10_midcheck",
                "Type": 1,
                "Command": "echo \"This is midcheck\"\nexit 0\n",
                "Argv": "--ignore-warring --ignore-error"
            },
            {
                "Name": "20_postcheck",
                "Type": 2,
                "Command": "echo \"This is postcheck\"\nexit 0\n",
                "Argv": "--ignore-warring --ignore-error"
            }
        ],
        "RepoInfo": [
            {
                "Name": "eagle/1060_main",
                "FilePath": "/tmp/Packages.1060_main",
                "HashSha256": "705206797cbe7b771b3b47366edacb097ee6c3c5e09d1b9958222d76705d626f"
            }
        ],
        "UUID": "c2ade74e-015c-49b2-8a7d-cb5767486e48",
        "TIme": "2023-10-10T15:34:42.233Z"
}`

	if err := runcmd.RunnerNotOutput(60, "wget", packageUrl, "-O", "/tmp/Packages.1060_main"); err != nil {
		return err
	}

	jsonfp, err := os.Create("/tmp/metacfg.json")
	if err != nil {
		return err
	}

	defer jsonfp.Close()

	if _, werr := jsonfp.WriteString(tmpJson); werr != nil {
		return werr
	}

	return nil
}

func TestSystemUpdate(t *testing.T) {

	os.Setenv("DEEPIN_SYSTEM_UPDATE_TOOLS_DEBUG", "debug")
	if err := createTestData(); err != nil {
		t.SkipNow()
	}

	if err := fs.CheckFileExistState("/tmp/metacfg.json"); err != nil {
		t.SkipNow()
	}

	if err := fs.CheckFileExistState("/tmp/Packages.1060_main"); err != nil {
		t.SkipNow()
	}

	defer os.Remove("/tmp/metacfg.json")
	defer os.Remove("/tmp/Packages.1060_main")
}
