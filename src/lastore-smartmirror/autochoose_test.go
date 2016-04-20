/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import "testing"
import C "gopkg.in/check.v1"

type testWrap struct{}

func Test(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&testWrap{})
}

func (*testWrap) TestAutoChoose(c *C.C) {
	official := "http://packages.deepin.com/deepin/"
	mirror := "http://cdn.packages.deepin.com/deepin/"
	invalid := "http://packages.deepin.com/experimetnal"

	u := official + "/dists/unstable/Release"
	c.Check(AutoChoose(u, official, mirror), C.Equals, u)
	u = official + "/dists/unstable/InRelease"
	c.Check(AutoChoose(u, official, mirror), C.Equals, u)
	u = official + "/dists/unstable/Release.gpg"
	c.Check(AutoChoose(u, official, mirror), C.Equals, u)

	u = official + "/dists/unstable/Release.gpg"
	c.Check(AutoChoose(u, invalid, mirror), C.Equals, u)
	u = official + "/dists/unstable/Release"
	c.Check(AutoChoose(u, invalid, mirror), C.Equals, u)

}
