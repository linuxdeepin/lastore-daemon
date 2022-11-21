// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLastoreSmartmirrorDaemon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LastoreSmartmirrorDaemon Suite")
}
