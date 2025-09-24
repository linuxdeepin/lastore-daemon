// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package check

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
)

func TestCheckDynHook(t *testing.T) {
	CheckDynHook(nil, cache.PreUpdate)
	CheckDynHook(nil, cache.MidCheck)
	CheckDynHook(nil, cache.PostCheck)
}
