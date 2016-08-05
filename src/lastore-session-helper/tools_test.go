/**
 * Copyright (C) 2016 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import (
	. "gopkg.in/check.v1"
	"testing"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type MySuite struct{}

var _ = Suite(&MySuite{})

func (*MySuite) TestStrSliceSetEqual(c *C) {
	c.Assert(strSliceSetEqual([]string{}, []string{}), Equals, true)
	c.Assert(strSliceSetEqual([]string{"a"}, []string{"a"}), Equals, true)
	c.Assert(strSliceSetEqual([]string{"a", "b"}, []string{"a"}), Equals, false)
	c.Assert(strSliceSetEqual([]string{"a", "b", "d"}, []string{"b", "d", "a"}), Equals, true)
}
