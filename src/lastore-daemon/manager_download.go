// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gettext"
)

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
		content, err := exec.Command("/bin/sh", []string{
			"-c",
			"df -BK --output='avail' /var|awk 'NR==2'",
		}...).CombinedOutput()
		if err != nil {
			logger.Warning(string(content))
		} else {
			spaceStr := strings.Replace(string(content), "K", "", -1)
			spaceStr = strings.TrimSpace(spaceStr)
			spaceNum, err := strconv.Atoi(spaceStr)
			if err != nil {
				logger.Warning(err)
			} else {
				spaceNum = spaceNum * 1000
				isInsufficientSpace = spaceNum < int(totalNeedDownloadSize)
			}
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
	job.initiator = initiator
	currentJob := job
	var sendDownloadingOnce sync.Once
	// 遍历job和所有next
	for currentJob != nil {
		j := currentJob
		currentJob = currentJob.next
		limitEnable, limitConfig := m.updater.GetLimitConfig()
		if limitEnable {
			j.option[aptLimitKey] = limitConfig
		}
		j.subRetryHookFn = func(job *Job) {
			// 下载限速的配置修改需要在job失败重试的时候修改配置(此处失败为手动终止设置的失败状态)
			m.handleDownloadLimitChanged(job)
		}
		j.realRunningHookFn = func() {
			m.PropsMu.Lock()
			m.PropsMu.Unlock()
			m.statusManager.SetUpdateStatus(mode, system.IsDownloading)
			if !m.updatePlatform.UpdateNowForce { // 立即更新则不发通知
				sendDownloadingOnce.Do(func() {
					msg := gettext.Tr("New version available! Downloading...")
					action := []string{
						"view",
						gettext.Tr("View"),
					}
					hints := map[string]dbus.Variant{"x-deepin-action-view": dbus.MakeVariant("dde-control-center,-m,update")}
					go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
				})
			}
			return
		}
		j.setPreHooks(map[string]func() error{
			string(system.PausedStatus): func() error {
				m.statusManager.SetUpdateStatus(mode, system.DownloadPause)
				return nil
			},
			string(system.FailedStatus): func() error {
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
						hints := map[string]dbus.Variant{"x-deepin-action-retry": dbus.MakeVariant("dde-control-center,-m,update")}
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
					m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
						Type:           "error",
						UpdateType:     mode.JobType(),
						JobDescription: job.Description,
						Detail:         fmt.Sprintf("download %v package failed, detail is %v", mode.JobType(), job.Description),
					}, true)
				}()
				return nil
			},
			string(system.SucceedStatus): func() error {
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
							hints := map[string]dbus.Variant{"x-deepin-action-updateNow": dbus.MakeVariant("dde-control-center,-m,update")}
							m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
						}
						m.reportLog(downloadStatusReport, true, "")
					}()

					if m.updatePlatform.UpdateNowForce {
						m.inhibitAutoQuitCountAdd()
						_, err := m.distUpgradePartly(dbus.Sender(m.service.Conn().Names()[0]), mode, true)
						if err != nil {
							logger.Error("failed to dist-upgrade:", err)
						}
						m.inhibitAutoQuitCountSub()
					}
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
