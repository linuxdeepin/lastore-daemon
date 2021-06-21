package main

import (
	"internal/system"
	"io/ioutil"
	"os"
	"path"
	"testing"

	C "gopkg.in/check.v1"
)

type configSuite struct{}

func TestConfig(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&configSuite{})
}

func (*configSuite) TestSuiteConfig(c *C.C) {
	// Suite临时文件测试config,Suite结束后会自动销毁c.MkDir()创建的目录
	dir := c.MkDir()
	tmpfile, err := ioutil.TempFile(dir, "config.json")
	c.Assert(err, C.Equals, nil)

	data, err := ioutil.ReadFile(path.Join(system.VarLibDir, configDataFilepath))
	if err != nil {
		if os.IsNotExist(err) {
			c.Log("TestSuiteConfig ReadFile error:", err)
			data = []byte("{\"filePath\":\"/\",\"Enable\":true}")
		} else {
			c.Assert(err, C.Equals, nil)
		}
	}

	err = ioutil.WriteFile(tmpfile.Name(), data, 0777)
	c.Assert(err, C.Equals, nil)

	configBefore := newConfig(tmpfile.Name())
	c.Assert(configBefore, C.NotNil)
	config := newConfig(tmpfile.Name())
	c.Assert(config, C.NotNil)

	err = config.setEnable(!config.Enable)
	c.Assert(err, C.Equals, nil)

	// 验证
	configAfter := newConfig(tmpfile.Name())
	c.Assert(configAfter, C.NotNil)
	c.Check(configAfter.Enable, C.Equals, !configBefore.Enable)
}
