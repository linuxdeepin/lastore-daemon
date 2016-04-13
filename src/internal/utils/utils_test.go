/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
package utils

import C "gopkg.in/check.v1"
import "testing"
import "os/exec"
import "time"
import "strings"
import "strconv"

type testWrap struct{}

func Test(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&testWrap{})
}

func (*testWrap) TestAppendSufix(c *C.C) {
	data := []struct {
		Raw    string
		Suffix string
		Result string
	}{
		{"", "a", "a"},
		{"raw", "", "raw"},
		{"raw", "raw", "raw"},
		{"raw", "rawraw", "rawrawraw"},
		{"raw", "suffix", "rawsuffix"},
		{"http://deepin.com/", "/", "http://deepin.com/"},
		{"http://deepin.com", "/", "http://deepin.com/"},
	}
	for _, d := range data {
		c.Check(AppendSuffix(d.Raw, d.Suffix), C.Equals, d.Result)
	}
}

func (*testWrap) TestExecOutput(c *C.C) {
	cmd := exec.Command("sh", "-c", "echo 1; echo 2; echo 4; echo 3")

	lines, err := FilterExecOutput(cmd, time.Millisecond*500, func(line string) bool {
		i, err := strconv.Atoi(line)
		if err != nil {
			return false
		}
		return i%2 == 0
	})

	c.Check(err, C.Equals, nil)
	c.Check(strings.Join(lines, " "), C.Equals, "2 4")
}
