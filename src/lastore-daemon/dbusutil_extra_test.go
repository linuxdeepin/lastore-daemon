// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/stretchr/testify/assert"

	"github.com/godbus/dbus/v5"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

func newTestService() *dbusutil.Service {
	return dbusutil.NewService(nil)
}

// --- Updater setProp / emitPropChanged tests ---

func TestUpdaterSetPropAutoCheckUpdates(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.False(t, u.setPropAutoCheckUpdates(false))
	assert.True(t, u.setPropAutoCheckUpdates(true))
	assert.Equal(t, true, u.AutoCheckUpdates)
	assert.False(t, u.setPropAutoCheckUpdates(true))
}

func TestUpdaterEmitPropChangedAutoCheckUpdates(t *testing.T) {
	u := &Updater{service: newTestService()}
	err := u.emitPropChangedAutoCheckUpdates(true)
	assert.Error(t, err)
}

func TestUpdaterSetPropAutoDownloadUpdates(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.True(t, u.setPropAutoDownloadUpdates(true))
	assert.Equal(t, true, u.AutoDownloadUpdates)
	assert.False(t, u.setPropAutoDownloadUpdates(true))
}

func TestUpdaterEmitPropChangedAutoDownloadUpdates(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedAutoDownloadUpdates(true))
}

func TestUpdaterSetPropUpdateNotify(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.True(t, u.setPropUpdateNotify(true))
	assert.Equal(t, true, u.UpdateNotify)
	assert.False(t, u.setPropUpdateNotify(true))
}

func TestUpdaterEmitPropChangedUpdateNotify(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedUpdateNotify(true))
}

func TestUpdaterSetPropMirrorSource(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.True(t, u.setPropMirrorSource("https://example.com"))
	assert.Equal(t, "https://example.com", u.MirrorSource)
	assert.False(t, u.setPropMirrorSource("https://example.com"))
}

func TestUpdaterEmitPropChangedMirrorSource(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedMirrorSource("test"))
}

func TestUpdaterSetPropUpdatableApps(t *testing.T) {
	u := &Updater{service: newTestService()}
	u.setPropUpdatableApps([]string{"pkg1", "pkg2"})
	assert.Equal(t, []string{"pkg1", "pkg2"}, u.UpdatableApps)
}

func TestUpdaterEmitPropChangedUpdatableApps(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedUpdatableApps([]string{"pkg1"}))
}

func TestUpdaterSetPropUpdatablePackages(t *testing.T) {
	u := &Updater{service: newTestService()}
	u.setPropUpdatablePackages([]string{"pkgA"})
	assert.Equal(t, []string{"pkgA"}, u.UpdatablePackages)
}

func TestUpdaterEmitPropChangedUpdatablePackages(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedUpdatablePackages([]string{"pkgA"}))
}

func TestUpdaterSetPropClassifiedUpdatablePackages(t *testing.T) {
	u := &Updater{service: newTestService()}
	val := map[string][]string{"system": {"pkg1"}}
	u.setPropClassifiedUpdatablePackages(val)
	assert.Equal(t, val, u.ClassifiedUpdatablePackages)
}

func TestUpdaterEmitPropChangedClassifiedUpdatablePackages(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedClassifiedUpdatablePackages(map[string][]string{}))
}

func TestUpdaterSetPropAutoInstallUpdates(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.True(t, u.setPropAutoInstallUpdates(true))
	assert.Equal(t, true, u.AutoInstallUpdates)
	assert.False(t, u.setPropAutoInstallUpdates(true))
}

func TestUpdaterEmitPropChangedAutoInstallUpdates(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedAutoInstallUpdates(true))
}

func TestUpdaterSetPropAutoInstallUpdateType(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.True(t, u.setPropAutoInstallUpdateType(system.SecurityUpdate))
	assert.Equal(t, system.SecurityUpdate, u.AutoInstallUpdateType)
	assert.False(t, u.setPropAutoInstallUpdateType(system.SecurityUpdate))
}

func TestUpdaterEmitPropChangedAutoInstallUpdateType(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedAutoInstallUpdateType(system.SecurityUpdate))
}

func TestUpdaterSetPropIdleDownloadConfig(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.True(t, u.setPropIdleDownloadConfig("config1"))
	assert.Equal(t, "config1", u.IdleDownloadConfig)
	assert.False(t, u.setPropIdleDownloadConfig("config1"))
}

func TestUpdaterEmitPropChangedIdleDownloadConfig(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedIdleDownloadConfig("test"))
}

func TestUpdaterSetPropDownloadSpeedLimitConfig(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.True(t, u.setPropDownloadSpeedLimitConfig("limit1"))
	assert.Equal(t, "limit1", u.DownloadSpeedLimitConfig)
	assert.False(t, u.setPropDownloadSpeedLimitConfig("limit1"))
}

func TestUpdaterEmitPropChangedDownloadSpeedLimitConfig(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedDownloadSpeedLimitConfig("test"))
}

func TestUpdaterSetPropUpdateTarget(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.True(t, u.setPropUpdateTarget("target1"))
	assert.Equal(t, "target1", u.UpdateTarget)
	assert.False(t, u.setPropUpdateTarget("target1"))
}

func TestUpdaterEmitPropChangedUpdateTarget(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedUpdateTarget("test"))
}

func TestUpdaterSetPropP2PUpdateEnable(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.True(t, u.setPropP2PUpdateEnable(true))
	assert.Equal(t, true, u.P2PUpdateEnable)
	assert.False(t, u.setPropP2PUpdateEnable(true))
}

func TestUpdaterEmitPropChangedP2PUpdateEnable(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedP2PUpdateEnable(true))
}

func TestUpdaterSetPropP2PUpdateSupport(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.True(t, u.setPropP2PUpdateSupport(true))
	assert.Equal(t, true, u.P2PUpdateSupport)
	assert.False(t, u.setPropP2PUpdateSupport(true))
}

func TestUpdaterEmitPropChangedP2PUpdateSupport(t *testing.T) {
	u := &Updater{service: newTestService()}
	assert.Error(t, u.emitPropChangedP2PUpdateSupport(true))
}

// --- Job setProp / emitPropChanged tests ---

func TestJobSetPropId(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.True(t, j.setPropId("job1"))
	assert.Equal(t, "job1", j.Id)
	assert.False(t, j.setPropId("job1"))
}

func TestJobEmitPropChangedId(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedId("job1"))
}

func TestJobSetPropName(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.True(t, j.setPropName("name1"))
	assert.Equal(t, "name1", j.Name)
	assert.False(t, j.setPropName("name1"))
}

func TestJobEmitPropChangedName(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedName("test"))
}

func TestJobSetPropPackages(t *testing.T) {
	j := &Job{service: newTestService()}
	j.setPropPackages([]string{"pkg1"})
	assert.Equal(t, []string{"pkg1"}, j.Packages)
}

func TestJobEmitPropChangedPackages(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedPackages([]string{"pkg1"}))
}

func TestJobSetPropCreateTime(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.True(t, j.setPropCreateTime(12345))
	assert.Equal(t, int64(12345), j.CreateTime)
	assert.False(t, j.setPropCreateTime(12345))
}

func TestJobEmitPropChangedCreateTime(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedCreateTime(12345))
}

func TestJobSetPropDownloadSize(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.True(t, j.setPropDownloadSize(9999))
	assert.Equal(t, int64(9999), j.DownloadSize)
	assert.False(t, j.setPropDownloadSize(9999))
}

func TestJobEmitPropChangedDownloadSize(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedDownloadSize(9999))
}

func TestJobEmitPropChangedUpdatePolicy(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedUpdatePolicy(1))
}

func TestJobSetPropType(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.True(t, j.setPropType("install"))
	assert.Equal(t, "install", j.Type)
	assert.False(t, j.setPropType("install"))
}

func TestJobEmitPropChangedType(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedType("install"))
}

func TestJobSetPropStatus(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.True(t, j.setPropStatus(system.RunningStatus))
	assert.Equal(t, system.RunningStatus, j.Status)
	assert.False(t, j.setPropStatus(system.RunningStatus))
}

func TestJobEmitPropChangedStatus(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedStatus(system.RunningStatus))
}

func TestJobSetPropProgress(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.True(t, j.setPropProgress(50.5))
	assert.Equal(t, 50.5, j.Progress)
	assert.False(t, j.setPropProgress(50.5))
}

func TestJobEmitPropChangedProgress(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedProgress(50.5))
}

func TestJobSetPropProto(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.True(t, j.setPropProto("http"))
	assert.Equal(t, "http", j.Proto)
	assert.False(t, j.setPropProto("http"))
}

func TestJobEmitPropChangedProto(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedProto("http"))
}

func TestJobSetPropDescription(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.True(t, j.setPropDescription("desc"))
	assert.Equal(t, "desc", j.Description)
	assert.False(t, j.setPropDescription("desc"))
}

func TestJobEmitPropChangedDescription(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedDescription("desc"))
}

func TestJobSetPropSpeed(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.True(t, j.setPropSpeed(1024))
	assert.Equal(t, int64(1024), j.Speed)
	assert.False(t, j.setPropSpeed(1024))
}

func TestJobEmitPropChangedSpeed(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedSpeed(1024))
}

func TestJobSetPropCancelable(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.True(t, j.setPropCancelable(true))
	assert.Equal(t, true, j.Cancelable)
	assert.False(t, j.setPropCancelable(true))
}

func TestJobEmitPropChangedCancelable(t *testing.T) {
	j := &Job{service: newTestService()}
	assert.Error(t, j.emitPropChangedCancelable(true))
}

// --- Manager setProp / emitPropChanged tests ---

func TestManagerSetPropJobList(t *testing.T) {
	m := &Manager{service: newTestService()}
	m.setPropJobList([]dbus.ObjectPath{"/org/deepin/dde/Lastore1/Job/1"})
	assert.Equal(t, []dbus.ObjectPath{"/org/deepin/dde/Lastore1/Job/1"}, m.JobList)
}

func TestManagerEmitPropChangedJobList(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.Error(t, m.emitPropChangedJobList([]dbus.ObjectPath{}))
}

func TestManagerSetPropUpgradableApps(t *testing.T) {
	m := &Manager{service: newTestService()}
	m.setPropUpgradableApps([]string{"app1"})
	assert.Equal(t, []string{"app1"}, m.UpgradableApps)
}

func TestManagerEmitPropChangedUpgradableApps(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.Error(t, m.emitPropChangedUpgradableApps([]string{"app1"}))
}

func TestManagerSetPropSystemOnChanging(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.True(t, m.setPropSystemOnChanging(true))
	assert.Equal(t, true, m.SystemOnChanging)
	assert.False(t, m.setPropSystemOnChanging(true))
}

func TestManagerEmitPropChangedSystemOnChanging(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.Error(t, m.emitPropChangedSystemOnChanging(true))
}

func TestManagerSetPropDownloadLimitOnChanging(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.True(t, m.setPropDownloadLimitOnChanging(true))
	assert.Equal(t, true, m.DownloadLimitOnChanging)
	assert.False(t, m.setPropDownloadLimitOnChanging(true))
}

func TestManagerEmitPropChangedDownloadLimitOnChanging(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.Error(t, m.emitPropChangedDownloadLimitOnChanging(true))
}

func TestManagerSetPropAutoClean(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.True(t, m.setPropAutoClean(true))
	assert.Equal(t, true, m.AutoClean)
	assert.False(t, m.setPropAutoClean(true))
}

func TestManagerEmitPropChangedAutoClean(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.Error(t, m.emitPropChangedAutoClean(true))
}

func TestManagerSetPropUpdateMode(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.True(t, m.setPropUpdateMode(system.SecurityUpdate))
	assert.Equal(t, system.SecurityUpdate, m.UpdateMode)
	assert.False(t, m.setPropUpdateMode(system.SecurityUpdate))
}

func TestManagerEmitPropChangedUpdateMode(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.Error(t, m.emitPropChangedUpdateMode(system.SecurityUpdate))
}

func TestManagerSetPropCheckUpdateMode(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.True(t, m.setPropCheckUpdateMode(system.SecurityUpdate))
	assert.Equal(t, system.SecurityUpdate, m.CheckUpdateMode)
	assert.False(t, m.setPropCheckUpdateMode(system.SecurityUpdate))
}

func TestManagerEmitPropChangedCheckUpdateMode(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.Error(t, m.emitPropChangedCheckUpdateMode(system.SecurityUpdate))
}

func TestManagerSetPropUpdateStatus(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.True(t, m.setPropUpdateStatus("updated"))
	assert.Equal(t, "updated", m.UpdateStatus)
	assert.False(t, m.setPropUpdateStatus("updated"))
}

func TestManagerEmitPropChangedUpdateStatus(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.Error(t, m.emitPropChangedUpdateStatus("test"))
}

func TestManagerSetPropHardwareId(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.True(t, m.setPropHardwareId("hw123"))
	assert.Equal(t, "hw123", m.HardwareId)
	assert.False(t, m.setPropHardwareId("hw123"))
}

func TestManagerEmitPropChangedHardwareId(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.Error(t, m.emitPropChangedHardwareId("hw123"))
}

func TestManagerSetPropImmutableAutoRecovery(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.True(t, m.setPropImmutableAutoRecovery(true))
	assert.Equal(t, true, m.ImmutableAutoRecovery)
	assert.False(t, m.setPropImmutableAutoRecovery(true))
}

func TestManagerEmitPropChangedImmutableAutoRecovery(t *testing.T) {
	m := &Manager{service: newTestService()}
	assert.Error(t, m.emitPropChangedImmutableAutoRecovery(true))
}
