// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/dut"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gettext"
)

const (
	timeFormat        = "15:04"
	timeFormatWithSec = "15:04:05"
)

func (m *Manager) setEffectiveOnlineRateLimit(nowTime string) {
	downloadSpeed := m.updater.downloadSpeedLimitConfigObj
	m.applyOnlineRateLimit(&downloadSpeed, nowTime)
	downloadSpeedStr, err := json.Marshal(downloadSpeed)
	if err == nil {
		logger.Infof("setEffectiveOnlineRateLimit %v --> %v", m.config.DownloadSpeedLimitConfig, string(downloadSpeedStr))
		m.updater.SetDownloadSpeedLimit(string(downloadSpeedStr))
	}
}

func (m *Manager) applyOnlineRateLimit(downloadSpeed *downloadSpeedLimitConfig, nowTime string) {
	onlineRateLimit := m.updatePlatform.OnlineRateLimit
	if onlineRateLimit.AllDayRateLimit.Enable {
		downloadSpeed.LimitSpeed = strconv.Itoa(onlineRateLimit.AllDayRateLimit.Bps)
		downloadSpeed.IsOnlineSpeedLimit = true
		downloadSpeed.DownloadSpeedLimitEnabled = true
	} else if isTimeInRange(nowTime, onlineRateLimit.PeakTimeRateLimit.StartTime, onlineRateLimit.PeakTimeRateLimit.EndTime) &&
		onlineRateLimit.PeakTimeRateLimit.Enable {
		downloadSpeed.LimitSpeed = strconv.Itoa(onlineRateLimit.PeakTimeRateLimit.Bps)
		downloadSpeed.IsOnlineSpeedLimit = true
		downloadSpeed.DownloadSpeedLimitEnabled = true
	} else if isTimeInRange(nowTime, onlineRateLimit.OffPeakTimeRateLimit.StartTime, onlineRateLimit.OffPeakTimeRateLimit.EndTime) &&
		onlineRateLimit.OffPeakTimeRateLimit.Enable {
		downloadSpeed.LimitSpeed = strconv.Itoa(onlineRateLimit.OffPeakTimeRateLimit.Bps)
		downloadSpeed.IsOnlineSpeedLimit = true
		downloadSpeed.DownloadSpeedLimitEnabled = true
	} else {
		err := json.Unmarshal([]byte(m.config.LocalDownloadSpeedLimitConfig), downloadSpeed)
		if err != nil {
			downloadSpeed.IsOnlineSpeedLimit = false
			downloadSpeed.LimitSpeed = strconv.FormatInt(defaultSpeedLimit, 10)
			downloadSpeed.DownloadSpeedLimitEnabled = true
		}
	}
}

func isTimeInRange(nowTimeStr, startStr, endStr string) bool {
	now, err := parseTime(nowTimeStr)
	if err != nil {
		return false
	}
	start, err := parseTime(startStr)
	if err != nil {
		return false
	}
	end, err := parseTime(endStr)
	if err != nil {
		return false
	}
	if start.Before(end) {
		return now.After(start) && now.Before(end)
	}
	return now.After(start) || now.Before(end)
}

func parseTime(t string) (time.Time, error) {
	if len(t) > 5 {
		return time.Parse(timeFormatWithSec, t)
	}
	return time.Parse(timeFormat, t)
}

// calculateTotalDownloadSize calculates the total download size for all packages under the specified update mode
func calculateTotalDownloadSize(mode system.UpdateType, updatablePkgsMap map[system.UpdateType][]string) (float64, []error) {
	totalNeedDownloadSize := 0.0
	var errs []error
	for _, updateType := range system.AllUpdateType() {
		if mode&updateType == 0 {
			continue
		}
		updatablePkgs := updatablePkgsMap[updateType]
		if needDownloadSize, _, err := system.QuerySourceDownloadSize(updateType, updatablePkgs); err == nil {
			totalNeedDownloadSize += needDownloadSize
		} else {
			errs = append(errs, err)
		}
	}
	return totalNeedDownloadSize, errs
}

func (m *Manager) prepareDistUpgrade(sender dbus.Sender, origin system.UpdateType, initiator Initiator) (*Job, error) {
	if m.ImmutableAutoRecovery {
		logger.Info("immutable auto recovery is enabled, don't allow to exec prepareDistUpgrade")
		return nil, errors.New("immutable auto recovery is enabled, don't allow to exec prepareDistUpgrade")
	}
	if !system.IsAuthorized() {
		return nil, errors.New("not authorized, don't allow to exec download")
	}
	environ, err := makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}
	m.ensureUpdateSourceOnce()
	m.updateJobList()
	var mode system.UpdateType
	// 如果获取到强制更新策略，那么忽略是否选中或者开启更新类型的状态
	if updateplatform.IsForceUpdate(m.updatePlatform.Tp) {
		mode = origin
	} else {
		mode = m.statusManager.GetCanPrepareDistUpgradeMode(origin) // 正在下载的状态会包含其中,会在创建job中找到对应job(由于不追加下载,因此直接返回之前的job) TODO 如果需要追加下载,需要根据前后path的差异,reload该job
		if mode == 0 {
			return nil, errors.New("don't exist can prepareDistUpgrade mode")
		}
	}

	packages, updatablePkgsMap := m.updater.getUpdatablePackagesWithClassification(mode)
	if len(packages) == 0 {
		return nil, system.NotFoundError("empty UpgradableApps")
	}

	// Calculate the total download size required
	totalNeedDownloadSize, _ := calculateTotalDownloadSize(mode, updatablePkgsMap)
	// 不再处理needDownloadSize == 0的情况,因为有可能是其他仓库包含了该仓库的包,导致该仓库无需下载,可以直接继续后续流程,用来切换该仓库的状态
	// 下载前检查/var分区的磁盘空间是否足够下载,
	isInsufficientSpace := false
	if totalNeedDownloadSize > 0 {
		spaceNum, err := system.GetFreeSpace("/var")
		if err != nil {
			logger.Warning(err)
		} else {
			isInsufficientSpace = spaceNum < int(totalNeedDownloadSize)
		}
	}

	if isInsufficientSpace {
		dbusError := system.JobError{
			ErrType:      system.ErrorInsufficientSpace,
			ErrDetail:    "You don't have enough free space to download",
			IsCheckError: true,
		}
		msg := fmt.Sprintf(gettext.Tr("Downloading updates failed. Please free up %g GB disk space first."), totalNeedDownloadSize/(1000*1000*1000))
		go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
		logger.Warning(dbusError.Error())
		errStr, _ := json.Marshal(dbusError)
		m.statusManager.SetUpdateStatus(mode, system.IsDownloading)
		m.statusManager.SetUpdateStatus(mode, system.DownloadErr)
		return nil, dbusutil.ToError(errors.New(string(errStr)))
	}
	done := make(chan bool)
	if m.config.IntranetUpdate {
		//私有化更新有忙闲时下载限速的功能，需要在真正开始下载前刷新一下线上限速配置
		if err = m.refreshThrottlingFromPlatform(); err != nil {
			logger.Warning("updatePlatform gen download speed limit failed", err)
		} else {
			go func() {
				ticker := time.NewTicker(5 * time.Second)
				startTime := time.Now()
				defer ticker.Stop()
				var count int
				layout := "15:04:05"
				for {
					select {
					case <-done:
						logger.Info("online rate limit ticker stopped")
						return
					case t := <-ticker.C:
						count++
						downloadStartServiceTime, err := time.ParseInLocation(layout, m.updatePlatform.OnlineRateLimit.ServerTime, time.Local)
						if err != nil {
							logger.Warningf("format OnlineRateLimit service time failed, %v", err)
							return
						}
						logger.Infof("downloadStartServiceTime %v", downloadStartServiceTime)
						nowTime := downloadStartServiceTime.Add(t.Sub(startTime))
						m.setEffectiveOnlineRateLimit(nowTime.Format(layout))
					}
				}
			}()
		}
	}

	var job *Job
	var isExist bool

	// 新的下载处理方式
	m.do.Lock()
	defer m.do.Unlock()
	{
		m.updater.PropsMu.Lock()
		option := map[string]interface{}{
			"UpdateMode":   mode,
			"DownloadSize": m.statusManager.GetAllUpdateModeDownloadSize(),
			"PackageMap":   m.updater.ClassifiedUpdatablePackages,
		}
		isExist, job, err = m.jobManager.CreateJob("", system.PrepareDistUpgradeJobType, m.coreList, environ, option)
		m.updater.PropsMu.Unlock()
	}
	if err != nil {
		logger.Warningf("Prepare DistUpgrade error: %v\n", err)
		return nil, err
	}
	if isExist {
		return job, nil
	}
	if m.config.IntranetUpdate {
		msg := gettext.Tr("New version available! The download of the update package will begin shortly")
		go m.sendNotify(updateNotifyShow, 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutPrivate)
		m.updatePlatform.PostProcessEventMessage(updateplatform.ProcessEvent{
			TaskID:       1,
			EventType:    updateplatform.StartDownload,
			EventStatus:  true,
			EventContent: msg,
		})
	}
	job.initiator = initiator
	currentJob := job
	var sendDownloadingOnce sync.Once
	// 遍历job和所有next
	for currentJob != nil {
		j := currentJob
		currentJob = currentJob.next
		limitEnable, limitConfig, limitIsOnline := m.updater.GetLimitConfig()
		logger.Infof("preDistUpgrade limitEnable: %v, limitConfig: %v, limitIsOnline: %v", limitEnable, limitConfig, limitIsOnline)
		if limitEnable || limitIsOnline {
			j.option[aptLimitKey] = limitConfig
		}
		j.subRetryHookFn = func(job *Job) {
			// 下载限速的配置修改需要在job失败重试的时候修改配置(此处失败为手动终止设置的失败状态)
			m.handleDownloadLimitChanged(job)
		}
		j.initDownloadSize(totalNeedDownloadSize)
		j.realRunningHookFn = func() {
			m.statusManager.SetUpdateStatus(mode, system.IsDownloading)
			if !m.updatePlatform.UpdateNowForce || m.config.IntranetUpdate { // 立即更新则不发通知
				sendDownloadingOnce.Do(func() {
					msg := gettext.Tr("New version available! Downloading...")
					action := []string{
						"view",
						gettext.Tr("View"),
					}
					if m.config.IntranetUpdate {
						go m.sendNotify(updateNotifyShow, 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutPrivate)
					} else {
						hints := map[string]dbus.Variant{"x-deepin-action-view": dbus.MakeVariant("dde-control-center,-m,update")}
						go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					}
				})
			}
		}
		j.setPreHooks(map[string]func() error{
			string(system.RunningStatus): func() error {
				checkType := dut.PreDownloadCheck
				if systemErr := dut.CheckSystem(checkType, nil); systemErr != nil {
					logger.Warning(systemErr)
					go func(err *system.JobError) {
						m.updatePlatform.PostProcessEventMessage(updateplatform.ProcessEvent{
							TaskID:       1,
							EventType:    updateplatform.PreDownloadCheck,
							EventStatus:  false,
							EventContent: "PreDownloadCheck failed",
						})
					}(systemErr)
				} else {
					go m.updatePlatform.PostProcessEventMessage(updateplatform.ProcessEvent{
						TaskID:       1,
						EventType:    updateplatform.PreDownloadCheck,
						EventStatus:  true,
						EventContent: fmt.Sprintf("%v success", checkType),
					})
				}

				return nil
			},
			string(system.PausedStatus): func() error {
				m.statusManager.SetUpdateStatus(mode, system.DownloadPause)
				return nil
			},
			string(system.FailedStatus): func() error {
				if m.config.IntranetUpdate {
					done <- true
					cacheFile := "/tmp/checkpolicy.cache"
					_ = os.RemoveAll(cacheFile)
				}
				// 失败的单独设置失败类型的状态,其他的还原成未下载(其中下载完成的由于限制不会被修改)
				m.statusManager.SetUpdateStatus(j.updateTyp, system.DownloadErr)
				m.statusManager.SetUpdateStatus(mode, system.NotDownload)
				var errorContent system.JobError
				err = json.Unmarshal([]byte(j.Description), &errorContent)
				if err == nil {
					if strings.Contains(errorContent.ErrType.String(), system.ErrorInsufficientSpace.String()) {
						_, updatablePkgsMap := m.updater.getUpdatablePackagesWithClassification(mode)
						size, errs := calculateTotalDownloadSize(mode, updatablePkgsMap)
						if size == 0 && len(errs) > 0 {
							size = totalNeedDownloadSize
						}
						if size > 0 {
							msg := fmt.Sprintf(gettext.Tr("Downloading updates failed. Please free up %g GB disk space first."), size/(1000*1000*1000))
							go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
						}
					} else if strings.Contains(errorContent.ErrType.String(), system.ErrorDamagePackage.String()) {
						// 下载更新失败，需要apt-get clean后重新下载
						cleanAllCache()
						msg := gettext.Tr("Updates failed: damaged files. Please update again.")
						action := []string{"retry", gettext.Tr("Try Again")}
						var hints map[string]dbus.Variant
						if m.config.IntranetUpdate {
							hints = map[string]dbus.Variant{"x-deepin-action-retry": dbus.MakeVariant("dde-control-center,-m,updateprivate")}
						} else {
							hints = map[string]dbus.Variant{"x-deepin-action-retry": dbus.MakeVariant("dde-control-center,-m,update")}
						}
						go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					} else if strings.Contains(errorContent.ErrType.String(), system.ErrorFetchFailed.String()) {
						// 网络原因下载更新失败
						msg := gettext.Tr("Downloading updates failed. Please check your network.")
						action := []string{"view", gettext.Tr("View")}
						hints := map[string]dbus.Variant{"x-deepin-action-view": dbus.MakeVariant("dde-control-center,-m,network")}
						go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					}
				}

				go func() {
					m.inhibitAutoQuitCountAdd()
					defer m.inhibitAutoQuitCountSub()
					m.reportLog(downloadStatusReport, false, j.Description)
					// 上报下载失败状态
					msg := fmt.Sprintf("download %v package failed, detail is %v", mode.JobType(), job.Description)
					m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
						Type:           "error",
						UpdateType:     mode.JobType(),
						JobDescription: job.Description,
						Detail:         msg,
					}, true)
					m.updatePlatform.PostProcessEventMessage(updateplatform.ProcessEvent{
						TaskID:       1,
						EventType:    updateplatform.StartDownload,
						EventStatus:  false,
						EventContent: msg,
					})
				}()

				checkType := dut.PostDownloadCheck
				if systemErr := dut.CheckSystem(checkType, nil); systemErr != nil {
					logger.Warning(systemErr)
					go func(err *system.JobError) {
						m.updatePlatform.PostProcessEventMessage(updateplatform.ProcessEvent{
							TaskID:       1,
							EventType:    updateplatform.PostDownloadCheck,
							EventStatus:  false,
							EventContent: "PostDownloadCheck failed",
						})
					}(systemErr)
				} else {
					go m.updatePlatform.PostProcessEventMessage(updateplatform.ProcessEvent{
						TaskID:       1,
						EventType:    updateplatform.PostDownloadCheck,
						EventStatus:  true,
						EventContent: fmt.Sprintf("%v success", checkType),
					})
				}

				return nil
			},
			string(system.SucceedStatus): func() error {
				if m.config.IntranetUpdate {
					done <- true
				}
				msg := fmt.Sprintf("download %v package success", j.updateTyp.JobType())
				m.updatePlatform.PostProcessEventMessage(updateplatform.ProcessEvent{
					TaskID:       1,
					EventType:    updateplatform.DownloadComplete,
					EventStatus:  true,
					EventContent: msg,
				}) // 上报下载成功状态
				logger.Infof("enter download job succeed callback, UpdateNowForce: %v", m.updatePlatform.UpdateNowForce)
				m.statusManager.SetUpdateStatus(j.updateTyp, system.CanUpgrade)
				if j.next == nil {
					go func() {
						m.inhibitAutoQuitCountAdd()
						defer m.inhibitAutoQuitCountSub()
						if !m.updatePlatform.UpdateNowForce {
							msg := gettext.Tr("Downloading completed. You can install updates when shutdown or reboot.")
							action := []string{
								"updateNow",
								gettext.Tr("Update Now"),
								"ignore",
								gettext.Tr("Dismiss"),
							}
							if m.config.IntranetUpdate {
								if m.updatePlatform.Tp == updateplatform.UpdateRegularly {
									timeStr := m.updatePlatform.UpdateTime.Format("15:04:05")
									if timeStr != m.updateTime {
										m.updateTime = timeStr
									}
									msg = fmt.Sprintf(gettext.Tr("Downloading completed. The computer will be updated at %s"), m.updateTime)
									go m.sendNotify(updateNotifyShow, 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutPrivate)
								}
								if m.updatePlatform.Tp == updateplatform.UpdateShutdown {
									m.sendNotify(updateNotifyShow, 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutPrivate)
								}
							} else {
								hints := map[string]dbus.Variant{"x-deepin-action-updateNow": dbus.MakeVariant("dde-control-center,-m,update")}

								m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
							}

						}
						m.reportLog(downloadStatusReport, true, "")
					}()

					checkType := dut.PostDownloadCheck
					if systemErr := dut.CheckSystem(checkType, nil); systemErr != nil {
						logger.Warning(systemErr)
						go func(err *system.JobError) {
							m.updatePlatform.PostProcessEventMessage(updateplatform.ProcessEvent{
								TaskID:       1,
								EventType:    updateplatform.PostDownloadCheck,
								EventStatus:  false,
								EventContent: "PostDownloadCheck failed",
							})
						}(systemErr)
					} else {
						go m.updatePlatform.PostProcessEventMessage(updateplatform.ProcessEvent{
							TaskID:       1,
							EventType:    updateplatform.PostDownloadCheck,
							EventStatus:  true,
							EventContent: fmt.Sprintf("%v success", checkType),
						})
					}

					if m.updatePlatform.UpdateNowForce {
						m.inhibitAutoQuitCountAdd()
						_, err := m.distUpgradePartly(dbus.Sender(m.service.Conn().Names()[0]), mode, true)
						if err != nil {
							logger.Error("failed to dist-upgrade:", err)
						}
						m.inhibitAutoQuitCountSub()
					}
				} else {
					logger.Infof("job next is not empty, job id: %v", job.next.Id)
				}

				// 上报下载成功状态
				m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
					Type:           "info",
					UpdateType:     j.updateTyp.JobType(),
					JobDescription: j.Description,
					Detail:         fmt.Sprintf("download %v package success", j.updateTyp.JobType()),
				}, false)
				return nil
			},
			string(system.EndStatus): func() error {
				if m.config.IntranetUpdate {
					done <- true
				}
				if j.next == nil {
					logger.Info("running in last end hook")
					// 如果出现单项失败,其他的状态需要修改,IsDownloading->notDownload
					// 如果已经有单项下载完成,然后取消下载,DownloadPause->notDownload
					m.statusManager.SetUpdateStatus(mode, system.NotDownload)
					// 除了下载失败和下载成功之外,之前的状态为 IsDownloading DownloadPause 的都通过size进行状态修正
					if j.Status != system.FailedStatus && j.Status != system.SucceedStatus {
						m.statusManager.updateModeStatusBySize(j.updateTyp, m.coreList)
					}
					m.statusManager.UpdateCheckCanUpgradeByEachStatus()
				}
				return nil
			},
		})
	}

	if err = m.jobManager.addJob(job); err != nil {
		return nil, err
	}
	return job, nil
}
