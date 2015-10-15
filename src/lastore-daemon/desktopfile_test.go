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
