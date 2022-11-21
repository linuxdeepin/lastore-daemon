// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var list = []string{
	"http://mirrors.163.com",
	"http://mirror.cedia.org.ec",
	"http://mirrors.hust.edu.cn",
	"http://www.ftp.saix.net",
	"http://deepin.ipacct.com",
	"http://mirrors.up.pt",
	"http://mirror.zetup.net",
}

var mq = MirrorQuality{
	QualityMap: make(QualityMap),
	reportList: make(chan []Report),
}

var _ = Describe("LastoreSmartmirrorDaemon", func() {
	It("sort list", func() {
		mq.setQuality(list[0], &Quality{
			AverageDelay: 100,
			DetectCount:  3,
		})
		mq.setQuality(list[1], &Quality{
			AverageDelay: 50,
			DetectCount:  2,
		})
		mq.setQuality(list[2], &Quality{
			AverageDelay: 300,
			DetectCount:  1,
		})

		result := []string{
			"http://www.ftp.saix.net",
			"http://deepin.ipacct.com",
			"http://mirrors.up.pt",
			"http://mirror.zetup.net",
			"http://mirrors.hust.edu.cn",
			"http://mirror.cedia.org.ec",
			"http://mirrors.163.com",
		}
		Expect(mq.mergeSort(list, mq.selectLessAccess)).To(Equal(result))

		result = []string{
			"http://mirror.cedia.org.ec",
			"http://mirrors.163.com",
			"http://mirrors.hust.edu.cn",
			"http://www.ftp.saix.net",
			"http://deepin.ipacct.com",
			"http://mirrors.up.pt",
			"http://mirror.zetup.net",
		}
		Expect(mq.mergeSort(list, mq.compare)).To(Equal(result))
	})

	// It("select mirror one", func() {
	// 	result := []string{
	// 		"http://mirrors.163.com",
	// 		"http://mirror.cedia.org.ec",
	// 		"http://mirrors.hust.edu.cn",
	// 		"http://www.ftp.saix.net",
	// 		"http://deepin.ipacct.com",
	// 	}
	// 	Expect(mq.detectSelectMirror(list)).To(Equal(result))
	// })

	// It("select mirror two", func() {
	// 	result := []string{
	// 		"http://mirrors.up.pt",
	// 		"http://mirror.zetup.net",
	// 		"http://mirrors.163.com",
	// 		"http://mirror.cedia.org.ec",
	// 		"http://mirrors.hust.edu.cn",
	// 	}
	// 	Expect(mq.detectSelectMirror(list)).To(Equal(result))
	// })

	// It("select mirror seven", func() {
	// 	result := []string{
	// 		"http://mirrors.163.com",
	// 		"http://mirror.cedia.org.ec",
	// 		"http://mirrors.hust.edu.cn",
	// 		"http://www.ftp.saix.net",
	// 		"http://deepin.ipacct.com",
	// 	}
	// 	mq.detectSelectMirror(list)
	// 	mq.detectSelectMirror(list)
	// 	mq.detectSelectMirror(list)
	// 	mq.detectSelectMirror(list)
	// 	mq.detectSelectMirror(list)
	// 	mq.detectSelectMirror(list)
	// 	Expect(mq.detectSelectMirror(list)).To(Equal(result))
	// })

	// It("SmartMirror init", func() {
	// 	s := newSmartMirror(nil)
	// 	// fmt.Println(s.mirrorQuality.sortSelectMirror(s.sourcesURL))
	// 	// fmt.Println(s.mirrorQuality.lessAccessSelectMirror(s.sourcesURL))
	// })
})
