// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

	"github.com/godbus/dbus/v5"
	grub2 "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.grub2"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	"github.com/linuxdeepin/go-lib/strv"
)

const (
	grubScriptFile = "/boot/grub/grub.cfg"
)

type bootEntry uint

const (
	normalBootEntry bootEntry = iota
	rollbackBootEntry
)

type grubManager struct {
	grub grub2.Grub2
}

func newGrubManager(sysBus *dbus.Conn, loop *dbusutil.SignalLoop) *grubManager {
	m := &grubManager{
		grub: grub2.NewGrub2(sysBus),
	}
	m.grub.InitSignalExt(loop, true)
	return m
}

// 下次启动默认进入第一个入口启动
func (m *grubManager) createTempGrubEntry() error {
	// loongarch 将GRUB引导加载程序安装到硬盘的引导扇区
	//arch, err := updateplatform.GetArchInfo()
	//if err != nil {
	//	logger.Warning(err)
	//} else {
	//	if arch == "loongarch64" {
	//		err = exec.Command("grub-install").Run()
	//		if err != nil {
	//			return err
	//		}
	//	}
	//}
	err := exec.Command("grub-reboot", "0").Run()
	if err != nil {
		return err
	}
	err = exec.Command("update-grub").Run()
	if err != nil {
		return err
	}
	return nil
}

// changeGrubDefaultEntry 设置grub默认入口(社区版可能不需要进行grub设置)
func (m *grubManager) changeGrubDefaultEntry(to bootEntry) error {
	var title string
	var err error
	switch to {
	//case rollbackBootEntry:
	//	title = system.GetGrubRollbackTitle(grubScriptFile)
	case normalBootEntry:
		title = system.GetGrubNormalTitle(grubScriptFile)
	default:
		logger.Info("unknown boot entry", to)
		return nil
	}
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("failed to get %v entry form %v", to, grubScriptFile)
	}
	curEntryTitle, err := m.grub.DefaultEntry().Get(0)
	if err != nil {
		logger.Warning(err)
		return err
	}
	// 如果DefaultEntry一样，就不再设置了，不然会下面的会超时
	if curEntryTitle == title {
		logger.Info("grub default entry need not to change:", curEntryTitle)
		return nil
	}
	logger.Info("try change grub default entry to:", title)
	entryTitles, err := m.grub.GetSimpleEntryTitles(0)
	if err != nil {
		logger.Warning(err)
		return err
	}
	if !strv.Strv(entryTitles).Contains(title) {
		return fmt.Errorf("grub no %s entry", title)
	}
	err = m.grub.SetDefaultEntry(0, title)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	wg.Add(1)
	logger.Info("updating grub default entry to ", title)
	timer := time.AfterFunc(30*time.Second, func() {
		logger.Warning("timeout while waiting for grub to update")
		wg.Done()
	})

	_ = m.grub.Updating().ConnectChanged(func(hasValue bool, updating bool) {
		if !hasValue {
			return
		}
		if !updating {
			logger.Info("successfully updated grub default entry to", title)
			wg.Done()
		}
	})
	wg.Wait()
	timer.Stop()
	m.grub.RemoveHandler(proxy.RemovePropertiesChangedHandler)
	return nil
}
