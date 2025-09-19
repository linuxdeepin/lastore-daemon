// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package check

import (
	"reflect"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
)

// AdjustPkgArchWithName
func TestAdjustPkgArchWithName(t *testing.T) {
	olddata := cache.CacheInfo{
		UpdateMetaInfo: cache.UpdateInfo{
			PkgList: []cache.AppInfo{
				cache.AppInfo{
					Name: "aaaa:i386",
				},
			},
		},
	}

	olddata2 := cache.CacheInfo{
		UpdateMetaInfo: cache.UpdateInfo{
			PkgList: []cache.AppInfo{
				cache.AppInfo{
					Name: "aaaa: i386 ",
				},
			},
		},
	}
	olddata3 := cache.CacheInfo{
		UpdateMetaInfo: cache.UpdateInfo{
			PkgList: []cache.AppInfo{
				cache.AppInfo{
					Name: "aaaa",
				},
				cache.AppInfo{
					Name: "aaaa:i386",
				},
			},
		},
	}

	olddata4 := cache.CacheInfo{
		UpdateMetaInfo: cache.UpdateInfo{
			PkgList: []cache.AppInfo{
				cache.AppInfo{
					Name: "aaaa",
					Arch: "amd64",
				},
			},
		},
	}

	newdata := cache.CacheInfo{
		UpdateMetaInfo: cache.UpdateInfo{
			PkgList: []cache.AppInfo{
				cache.AppInfo{
					Name: "aaaa",
					Arch: "i386",
				},
			},
		},
	}
	newdata2 := cache.CacheInfo{
		UpdateMetaInfo: cache.UpdateInfo{
			PkgList: []cache.AppInfo{
				cache.AppInfo{
					Name: "aaaa",
				},
				cache.AppInfo{
					Name: "aaaa",
					Arch: "i386",
				},
			},
		},
	}

	newdata3 := cache.CacheInfo{
		UpdateMetaInfo: cache.UpdateInfo{
			PkgList: []cache.AppInfo{
				cache.AppInfo{
					Name: "aaaa",
					Arch: "amd64",
				},
			},
		},
	}

	Warp := func(oldd, newd *cache.CacheInfo) {

		AdjustPkgArchWithName(oldd)

		if !reflect.DeepEqual(oldd, newd) {
			t.Logf("old:%v \nnew: %v", oldd, newd)
			t.Error("AdjustPkgArchWithName not equal")
		}
	}

	Warp(&olddata, &newdata)

	Warp(&olddata2, &newdata)

	Warp(&olddata3, &newdata2)

	Warp(&olddata4, &newdata3)

}

func TestCheckDynHook(t *testing.T) {
	CheckDynHook(nil, cache.PreUpdate)
	CheckDynHook(nil, cache.UpdateCheck)
	CheckDynHook(nil, cache.PostCheck)
}
