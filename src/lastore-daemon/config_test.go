package main

import (
	"internal/system"
	"io/ioutil"
	"os"
	"path"
	"time"

	C "gopkg.in/check.v1"
)

type configSuite struct{}

func init() {
	C.Suite(&configSuite{})
}

func (*configSuite) TestSuiteConfig(c *C.C) {
	// Suite临时文件测试config,Suite结束后会自动销毁c.MkDir()创建的目录
	dir := c.MkDir()
	tmpfile, err := ioutil.TempFile(dir, "config.json")
	c.Assert(err, C.Equals, nil)
	data, err := ioutil.ReadFile(path.Join(system.VarLibDir, "config.json"))
	if err != nil {
		if os.IsNotExist(err) {
			c.Log("TestSuiteConfig ReadFile error:", err)
			data = []byte("{\"Version\":\"0.1\",\"AutoCheckUpdates\":true,\"DisableUpdateMetadata\":false,\"AutoDownloadUpdates\":false,\"AutoClean\":true,\"MirrorSource\":\"default\",\"UpdateNotify\":true,\"CheckInterval\":604800000000000,\"CleanInterval\":604800000000000,\"UpdateMode\":3,\"CleanIntervalCacheOverLimit\":86400000000000,\"AppstoreRegion\":\"\",\"LastCheckTime\":\"2021-06-17T14:10:21.896021304+08:00\",\"LastCleanTime\":\"2021-06-17T09:18:31.515019638+08:00\",\"LastCheckCacheSizeTime\":\"2021-06-17T09:18:31.5151104+08:00\",\"Repository\":\"desktop\",\"MirrorsUrl\":\"http://packages.deepin.com/mirrors/community.json\",\"AllowInstallRemovePkgExecPaths\":null}")
		} else {
			c.Assert(err, C.Equals, nil)
		}
	}
	err = ioutil.WriteFile(tmpfile.Name(), data, 0777)
	c.Assert(err, C.Equals, nil)

	configBefore := NewConfig(tmpfile.Name())
	c.Assert(configBefore, C.NotNil)
	config := NewConfig(tmpfile.Name())
	c.Assert(config, C.NotNil)

	time.Sleep(time.Millisecond * 10)
	err = config.UpdateLastCheckTime()
	c.Assert(err, C.Equals, nil)
	err = config.UpdateLastCleanTime()
	c.Assert(err, C.Equals, nil)
	err = config.UpdateLastCheckCacheSizeTime()
	c.Assert(err, C.Equals, nil)
	err = config.SetAutoCheckUpdates(!config.AutoCheckUpdates)
	c.Assert(err, C.Equals, nil)
	err = config.SetUpdateNotify(!config.UpdateNotify)
	c.Assert(err, C.Equals, nil)
	err = config.SetAutoDownloadUpdates(!config.AutoDownloadUpdates)
	c.Assert(err, C.Equals, nil)
	err = config.SetAutoClean(!config.AutoClean)
	c.Assert(err, C.Equals, nil)
	err = config.SetMirrorSource(config.MirrorSource + "Test")
	c.Assert(err, C.Equals, nil)
	err = config.SetAppstoreRegion(config.AppstoreRegion + "Test")
	c.Assert(err, C.Equals, nil)
	err = config.SetUpdateMode(config.UpdateMode + 1)
	c.Assert(err, C.Equals, nil)

	// 验证
	configAfter := NewConfig(tmpfile.Name())
	c.Assert(configAfter, C.NotNil)
	c.Check(configAfter.LastCheckTime, C.Not(C.Equals), configBefore.LastCheckTime)
	c.Check(configAfter.LastCleanTime, C.Not(C.Equals), configBefore.LastCleanTime)
	c.Check(configAfter.LastCheckCacheSizeTime, C.Not(C.Equals), configBefore.LastCheckCacheSizeTime)
	c.Check(configAfter.AutoCheckUpdates, C.Equals, !configBefore.AutoCheckUpdates)
	c.Check(configAfter.UpdateNotify, C.Equals, !configBefore.UpdateNotify)
	c.Check(configAfter.AutoDownloadUpdates, C.Equals, !configBefore.AutoDownloadUpdates)
	c.Check(configAfter.AutoClean, C.Equals, !configBefore.AutoClean)
	c.Check(configAfter.MirrorSource, C.Equals, configBefore.MirrorSource+"Test")
	c.Check(configAfter.AppstoreRegion, C.Equals, configBefore.AppstoreRegion+"Test")
	c.Check(configAfter.UpdateMode, C.Equals, configBefore.UpdateMode+1)
}
