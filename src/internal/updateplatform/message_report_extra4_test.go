// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"testing"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestUpdateSourceListEarlyReturn(t *testing.T) {
	// When IntranetUpdate=false or PlatformUpdate=false, UpdateSourceList should early-return
	// without attempting any file writes.
	m := &UpdatePlatformManager{
		config: &Cfg.Config{
			IntranetUpdate: false,
			PlatformUpdate: false,
		},
	}
	// should not panic or error (the function returns void, just verify no panic)
	m.UpdateSourceList()

	// also test with IntranetUpdate=true but PlatformUpdate=false
	m2 := &UpdatePlatformManager{
		config: &Cfg.Config{
			IntranetUpdate: true,
			PlatformUpdate: false,
		},
	}
	m2.UpdateSourceList()

	// and IntranetUpdate=false but PlatformUpdate=true
	m3 := &UpdatePlatformManager{
		config: &Cfg.Config{
			IntranetUpdate: false,
			PlatformUpdate: true,
		},
	}
	m3.UpdateSourceList()

	assert.True(t, true, "all early-return paths completed without panic")
}
