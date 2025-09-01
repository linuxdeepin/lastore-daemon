// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package repocache

import (
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"

	// "runtime"

	"testing"
)

func TestDebRepo(t *testing.T) {
	t.Parallel()

	// memFile, err := os.Create("mem.pprof")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer memFile.Close()

	repo := DebRepo{}
	repo.CacheRef = make(map[string]string, 3)
	repo.AppinfoHash = make(map[string]cache.AppInfo, 1000)

	repo.RepoToCache("/opt/uos-update/cache/repo",
		"xxxxs://example.com/testapp",
		"eagle/1050",
		"main")

	repo.RepoToCache("/opt/uos-update/cache/repo",
		"xxxxs://example.com/testapp",
		"eagle/1050",
		"contrib")

	repo.RepoToCache("/opt/uos-update/cache/repo",
		"xxxxs://example.com/testapp",
		"eagle/1050",
		"non-free")

	log.Debugf("sum:%d", len(repo.AppinfoHash))
	log.Debugf("cacheref:%+v", repo.CacheRef)

	// repo.AppinfoHash = nil
	// repo.AppinfoHash = make(map[string]cache.AppInfo, 1000)

	if err := repo.Dump("/opt/uos-update/cache/repo/repo.yaml"); err != nil {
		log.Debugf("err:+%v", err)
	}

	log.Debugf("sum:%d", len(repo.AppinfoHash))
	log.Debugf("cacheref:%+v", repo.CacheRef)

	repo.AppinfoHash = nil
	repo.CacheRef = nil
	repo.AppinfoHash = make(map[string]cache.AppInfo, 1000)
	repo.CacheRef = make(map[string]string, 3)

	// repo.LoaderCache("/opt/uos-update/cache/repo/main/d1d12d161ce32070dd6906f3ebb6d4d91c163983.gob")
	// repo.LoaderCache("/opt/uos-update/cache/repo/contrib/fd59e1dbd71209de23ccc37aa5a3fa77751ab4ee.gob")
	// repo.LoaderCache("/opt/uos-update/cache/repo/non-free/3a41cb9087078f0ad9e812d81c5e46492e07a471.gob")
	if err := repo.Loader("/opt/uos-update/cache/repo/repo.yaml"); err != nil {
		log.Debugf("err:+%v", err)
	}
	log.Debugf("sum:%d", len(repo.AppinfoHash))
	log.Debugf("cacheref:%+v", repo.CacheRef)
	//runtime.GC()
	// pprof.WriteHeapProfile(memFile)
	// for _, tds := range testDataSet1 {
	// 	ret := DialUrlHttpGet(tds.in, 5)
	// 	retB := (ret == nil)
	// 	if retB != tds.out {
	// 		t.Errorf("the key %v , ret %v", tds, ret)
	// 	}
	// }
}
