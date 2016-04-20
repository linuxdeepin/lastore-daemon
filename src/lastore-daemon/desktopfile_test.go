/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import C "gopkg.in/check.v1"

func (*testWrap) TestDesktopBestOne(c *C.C) {
	data := []struct {
		Files   []string
		BestOne int
	}{
		{
			[]string{
				"/usr/share/plasma/plasmoids/org.kde.plasma.katesessions/metadata.desktop",
				"/usr/share/applications/org.kde.kate.desktop",
			}, 1,
		},
	}

	for _, item := range data {
		c.Check(DesktopFiles(item.Files).BestOne(), C.Equals, item.Files[item.BestOne])
	}
}
