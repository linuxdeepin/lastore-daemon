// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestGenRules(t *testing.T) {
	tmp := &PreInstalledPkgMeta{}
	var preChecks, midChecks, postChecks []ShellCheck

	for i := 0; i < 3; i++ {
		Check := ShellCheck{
			Name:  fmt.Sprintf("%d_test_pre_check.sh", i),
			Shell: "IyEvYmluL2Jhc2gKCmVjaG8gImhlbGxvIGRlZXBpbiI",
		}
		preChecks = append(preChecks, Check)
	}

	for i := 0; i < 3; i++ {
		Check := ShellCheck{
			Name:  fmt.Sprintf("%d_test_mid_check.sh", i),
			Shell: "IyEvYmluL2Jhc2gKCmVjaG8gImhlbGxvIGRlZXBpbiI",
		}
		midChecks = append(midChecks, Check)
	}

	for i := 0; i < 3; i++ {
		Check := ShellCheck{
			Name:  fmt.Sprintf("%d_test_post_check.sh", i),
			Shell: "IyEvYmluL2Jhc2gKCmVjaG8gImhlbGxvIGRlZXBpbiI",
		}
		postChecks = append(postChecks, Check)
	}

	tmp.PreCheck = preChecks
	tmp.MidCheck = midChecks
	tmp.PostCheck = postChecks

	tmp.Packages.Core = append(tmp.Packages.Core, system.PlatformPackageInfo{
		Name: "deepin.com.deepin.core",
		AllArchVersion: []system.Version{
			{
				Version: "1.0.0",
				Arch:    "amd64",
			},
		},
		Need: "strict",
	})

	tmp.Packages.Select = append(tmp.Packages.Select, system.PlatformPackageInfo{
		Name: "deepin.com.deepin.Select",
		AllArchVersion: []system.Version{
			{
				Version: "1.0.0",
				Arch:    "amd64",
			},
		},
		Need: "strict",
	})

	tmp.Packages.Freeze = append(tmp.Packages.Freeze, system.PlatformPackageInfo{
		Name: "deepin.com.deepin.Freeze",
		AllArchVersion: []system.Version{
			{
				Version: "1.0.0",
				Arch:    "amd64",
			},
		},
		Need: "strict",
	})

	tmp.Packages.Purge = append(tmp.Packages.Purge, system.PlatformPackageInfo{
		Name: "deepin.com.deepin.Purge",
		AllArchVersion: []system.Version{
			{
				Version: "1.0.0",
				Arch:    "amd64",
			},
		},
		Need: "strict",
	})

	str, err := json.Marshal(tmp)
	if err != nil {
		t.Error(err)
	}
	//fmt.Println(string(str))

	context, err := base64.RawStdEncoding.DecodeString("IyEvYmluL2Jhc2gKCmVjaG8gImhlbGxvIGRlZXBpbiI")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, string(context), "#!/bin/bash\n\necho \"hello deepin\"")

	fmt.Println(string(context))

	pkgs := getTargetPkgListData(str)
	upm := &UpdatePlatformManager{
		PreCheck:  pkgs.PreCheck,
		MidCheck:  pkgs.MidCheck,
		PostCheck: pkgs.PostCheck,
	}

	upm.GetRules()

}
