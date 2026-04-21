package ratelimit

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("lastore/rateLimit")

const UPGRADE_DELIVERY_SERVICE = "org.deepin.upgradedelivery"
const UPGRADE_DELIVERY_OBJECT_PATH = "/org/deepin/upgradedelivery"
const UPGRADE_DELIVERY_INTERFACE = "org.deepin.upgradedelivery"

const DefaultRateLimit = 10 * 1024 // 10 kb/s
const MinRateLimit = 10 * 1024     // 10 kb/s
const MaxRateLimit = 999999 * 1024 // 999999 kb/s

// SyncLimit 服务器端限速配置信息
type SyncLimit struct {
	AllDayRateLimit   *RateLimitWithTime `json:"a,omitempty"` // 全天限制
	FreeTimeRateLimit *RateLimitWithTime `json:"f,omitempty"` // 闲时限制
	BusyTimeRateLimit *RateLimitWithTime `json:"b,omitempty"` // 忙时限制
}

// RateLimitWithTime 时间区间限速
type RateLimitWithTime struct {
	StartTime string `json:"s,omitempty"` // 限制开始时间,format 22:00:00
	EndTime   string `json:"e,omitempty"` // 限制结束时间
	RateLimit int    `json:"r,omitempty"` // 最大下载速率 (单位: kb/s)
	Type      int    `json:"t,omitempty"` // 忙闲，1：选择，2：不选择
}

// IPFSConfig 远程服务器端配置文件信息
type IPFSConfig struct {
	ID            string     `json:"id"` // 配置文件ID
	DownloadLimit *SyncLimit `json:"dl"` // 下载限速
	UploadLimit   *SyncLimit `json:"ul"` // 上传限速
}

const (
	RateLimitTypeNo     = 0 // 表示不设置
	RateLimitTypeLocal  = 1 // 本地设置限速
	RateLimitTypeRemote = 2 // 远程设置限速(服务器下发策略)
)

// 当RateInfo中的LimitType为不限速时，LimitRate和CurrentRate需要设置成一个有效值即可，默认情况设置成DefaultRateLimit即可满足要求
type RateInfo struct {
	LimitType   int       // 限速类型(CLimitTypeNo,CLimitTypeLocal,CLimitTypeRemote)
	StartTime   time.Time // 限速开始时间
	EndTime     time.Time // 限速结束时间
	LimitRate   int64     // 限制速率(不限速前设置的速率值)
	CurrentRate int64     // 当前限制速率(实际限速速率：限速时，两者一致，不限速时，CurrentRate为最大限速速率)
}

type RateInfoEvent struct {
	RateInfo       // 生效速率信息
	RateType int   // 类型(gloabl、busy、free)
	Speed    int64 // 速度
}

type IPFSLimitRate struct {
	GlobalLimitRemote *RateInfo // 全局限速(服务器)
	GlobalLimitLocal  *RateInfo // 全局限速(本地)
	BusyLimitRemote   *RateInfo // 忙时限速(服务器)
	BusyLimitLocal    *RateInfo // 忙时限速(本地)
	FreeLimitRemote   *RateInfo // 空闲限速(服务器)
	FreeLimitLocal    *RateInfo // 空闲限速(本地)
}

func SetIPFSRateLimit(uploadLimitRate, downloadLimitRate IPFSLimitRate) error {
	// delivery dbus输入参数要求：
	// 1. GlobalLimitLocal必须有数据，且不能为空，如果不限速需要设置限速类型为0,限速值使用默认值
	// 2. 其他的限速，如果不限速，需要设置成nil
	if uploadLimitRate.GlobalLimitRemote != nil && uploadLimitRate.GlobalLimitRemote.LimitType == 0 {
		uploadLimitRate.GlobalLimitRemote = nil
	}
	if uploadLimitRate.BusyLimitRemote != nil && uploadLimitRate.BusyLimitRemote.LimitType == 0 {
		uploadLimitRate.BusyLimitRemote = nil
	}
	if uploadLimitRate.FreeLimitRemote != nil && uploadLimitRate.FreeLimitRemote.LimitType == 0 {
		uploadLimitRate.FreeLimitRemote = nil
	}
	if uploadLimitRate.BusyLimitLocal != nil && uploadLimitRate.BusyLimitLocal.LimitType == 0 {
		uploadLimitRate.BusyLimitLocal = nil
	}
	if uploadLimitRate.FreeLimitLocal != nil && uploadLimitRate.FreeLimitLocal.LimitType == 0 {
		uploadLimitRate.FreeLimitLocal = nil
	}

	if downloadLimitRate.GlobalLimitRemote != nil && downloadLimitRate.GlobalLimitRemote.LimitType == 0 {
		downloadLimitRate.GlobalLimitRemote = nil
	}
	if downloadLimitRate.BusyLimitRemote != nil && downloadLimitRate.BusyLimitRemote.LimitType == 0 {
		downloadLimitRate.BusyLimitRemote = nil
	}
	if downloadLimitRate.FreeLimitRemote != nil && downloadLimitRate.FreeLimitRemote.LimitType == 0 {
		downloadLimitRate.FreeLimitRemote = nil
	}
	if downloadLimitRate.BusyLimitLocal != nil && downloadLimitRate.BusyLimitLocal.LimitType == 0 {
		downloadLimitRate.BusyLimitLocal = nil
	}
	if downloadLimitRate.FreeLimitLocal != nil && downloadLimitRate.FreeLimitLocal.LimitType == 0 {
		downloadLimitRate.FreeLimitLocal = nil
	}

	ipfsUploadConfigData, err := json.Marshal(uploadLimitRate)
	if err != nil {
		return fmt.Errorf("failed to marshal upload limit rate: %w", err)
	}
	ipfsDownloadConfigData, err := json.Marshal(downloadLimitRate)
	if err != nil {
		return fmt.Errorf("failed to marshal download limit rate: %w", err)
	}
	logger.Infof("ipfsUploadConfigData: %s", string(ipfsUploadConfigData))
	logger.Infof("ipfsDownloadConfigData: %s", string(ipfsDownloadConfigData))
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("failed to connect to system bus: %w", err)
	}
	object := sysBus.Object(UPGRADE_DELIVERY_SERVICE, UPGRADE_DELIVERY_OBJECT_PATH)
	if err := object.Call(UPGRADE_DELIVERY_INTERFACE+".SetRateLimit", 0, string(ipfsUploadConfigData), string(ipfsDownloadConfigData)).Store(); err != nil {
		return fmt.Errorf("failed to set rate limit: %w", err)
	}
	return nil
}

// SetIPFSDownloadRateLimit sets the download rate limit for IPFS.
// rate is in kilobits per second (kb/s). If rate is -1, it means no rate limit.
func SetIPFSDownloadRateLimit(rate int) error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("failed to connect to system bus: %w", err)
	}
	object := sysBus.Object(UPGRADE_DELIVERY_SERVICE, UPGRADE_DELIVERY_OBJECT_PATH)
	if err := object.Call(UPGRADE_DELIVERY_INTERFACE+".SetDownloadRateLimit", 0, rate).Store(); err != nil {
		return fmt.Errorf("failed to set download rate limit: %w", err)
	}
	return nil
}

// SetIPFSUploadRateLimit sets the upload rate limit for IPFS.
// rate is in kilobits per second (kb/s). If rate is -1, it means no rate limit.
func SetIPFSUploadRateLimit(rate int) error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("failed to connect to system bus: %w", err)
	}
	object := sysBus.Object(UPGRADE_DELIVERY_SERVICE, UPGRADE_DELIVERY_OBJECT_PATH)
	if err := object.Call(UPGRADE_DELIVERY_INTERFACE+".SetUploadRateLimit", 0, rate).Store(); err != nil {
		return fmt.Errorf("failed to set upload rate limit: %w", err)
	}
	return nil
}

func GetDeliveryUploadRateLimit() (RateInfoEvent, error) {
	return getDeliveryRateLimit("UploadLimitSpeed")
}

func GetDeliveryDownloadRateLimit() (RateInfoEvent, error) {
	return getDeliveryRateLimit("DownloadLimitSpeed")
}

func getDeliveryRateLimit(method string) (RateInfoEvent, error) {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return RateInfoEvent{}, fmt.Errorf("failed to get delivery rate limit: %w", err)
	}
	object := sysBus.Object(UPGRADE_DELIVERY_SERVICE, UPGRADE_DELIVERY_OBJECT_PATH)
	limitSpeedData, err := object.GetProperty(UPGRADE_DELIVERY_INTERFACE + "." + method)
	if err != nil {
		return RateInfoEvent{}, fmt.Errorf("failed to get limit speed: %w", err)
	}

	var rateInfo RateInfoEvent
	limitSpeed := limitSpeedData.Value().(string)
	if limitSpeed == "" {
		return RateInfoEvent{}, fmt.Errorf("limit speed is empty")
	}
	if err := json.Unmarshal([]byte(limitSpeed), &rateInfo); err != nil {
		return RateInfoEvent{}, fmt.Errorf("failed to unmarshal limit speed: %w", err)
	}
	return rateInfo, nil
}

func GetIPFSLimitRateBySyncLimit(syncLimit SyncLimit) (IPFSLimitRate, error) {
	var limitRate IPFSLimitRate
	if syncLimit.AllDayRateLimit != nil {
		rateInfo := convertRateLimitWithTimeToRateInfo(syncLimit.AllDayRateLimit)
		if rateInfo != nil {
			limitRate.GlobalLimitRemote = rateInfo
		}
	}
	if syncLimit.BusyTimeRateLimit != nil && syncLimit.BusyTimeRateLimit.Type == 1 {
		rateInfo := convertRateLimitWithTimeToRateInfo(syncLimit.BusyTimeRateLimit)
		if rateInfo != nil {
			limitRate.BusyLimitRemote = rateInfo
		}
	}
	if syncLimit.FreeTimeRateLimit != nil && syncLimit.FreeTimeRateLimit.Type == 1 {
		rateInfo := convertRateLimitWithTimeToRateInfo(syncLimit.FreeTimeRateLimit)
		if rateInfo != nil {
			limitRate.FreeLimitRemote = rateInfo
		}
	}
	return limitRate, nil
}

func convertRateLimitWithTimeToRateInfo(rlwt *RateLimitWithTime) *RateInfo {
	if rlwt == nil {
		return nil
	}
	var rateInfo RateInfo
	if rlwt.Type == 1 {
		rateInfo.LimitType = RateLimitTypeRemote
		rateInfo.LimitRate = int64(rlwt.RateLimit) * 1024   // kb ---> byte
		rateInfo.CurrentRate = int64(rlwt.RateLimit) * 1024 // kb ---> byte
	} else {
		rateInfo.LimitType = RateLimitTypeNo
		rateInfo.LimitRate = DefaultRateLimit
		rateInfo.CurrentRate = DefaultRateLimit
	}

	if rlwt.StartTime != "" {
		startTime, err := time.Parse("15:04:05", rlwt.StartTime)
		if err == nil {
			rateInfo.StartTime = startTime
		}
	}
	if rlwt.EndTime != "" {
		endTime, err := time.Parse("15:04:05", rlwt.EndTime)
		if err == nil {
			rateInfo.EndTime = endTime
		}
	}
	return &rateInfo
}

type LocalRateLimitConfig struct {
	Global *RateInfo
	Busy   *RateInfo
	Free   *RateInfo
}

func ValidateRateInfo(rate *RateInfo) {
	if rate == nil {
		return
	}
	if rate.LimitRate < MinRateLimit || rate.LimitRate > MaxRateLimit {
		rate.LimitRate = DefaultRateLimit
	}
	if rate.CurrentRate < MinRateLimit || rate.CurrentRate > MaxRateLimit {
		rate.CurrentRate = DefaultRateLimit
	}
}

func (c *LocalRateLimitConfig) Validate() {
	ValidateRateInfo(c.Global)
	ValidateRateInfo(c.Busy)
	ValidateRateInfo(c.Free)
}

func GetLocalRateLimitFromConfig(globalLimit, peakLimit, offPeakLimit string) *LocalRateLimitConfig {
	var config LocalRateLimitConfig

	if globalLimit != "" {
		var rateInfo RateInfo
		if err := json.Unmarshal([]byte(globalLimit), &rateInfo); err == nil {
			config.Global = &rateInfo
		}
	}
	if peakLimit != "" {
		var rateInfo RateInfo
		if err := json.Unmarshal([]byte(peakLimit), &rateInfo); err == nil {
			config.Busy = &rateInfo
		}
	}
	if offPeakLimit != "" {
		var rateInfo RateInfo
		if err := json.Unmarshal([]byte(offPeakLimit), &rateInfo); err == nil {
			config.Free = &rateInfo
		}
	}

	return &config
}
