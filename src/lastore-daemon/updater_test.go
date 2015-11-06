package main

import C "gopkg.in/check.v1"
import "internal/system/apt"

func (*testWrap) TestMirrors(c *C.C) {

	ServerAPI = "http://repository.api.deepin.test"
	b := apt.New()
	updater := NewUpdater(b)

	list, err := updater.ListMirrorSources("zh_XX")

	c.Check(err, C.Equals, nil)
	c.Check(len(list), C.Not(C.Equals), 0)

	for _, mirror := range list {
		err := updater.SetMirrorSource(mirror.Id)
		c.Check(err, C.Equals, nil)

		//		u := NewUpdater(b)
		u := updater
		c.Check(u.MirrorSource, C.Equals, mirror.Id)
	}
}
