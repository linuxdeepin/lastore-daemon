package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// requestType 请求类型枚举
type requestType uint

const (
	GetVersion requestType = iota
	GetUpdateLog
	GetTargetPkgLists // 系统软件包清单
	GetCurrentPkgLists
	GetPkgCVEs // CVE 信息
	PostProcess
	PostProcessEvent
	PostResult
)

func (r requestType) string() string {
	return fmt.Sprintf("%v %v", Urls[r].method, Urls[r].path)
}

// requestContent 请求内容
type requestContent struct {
	path   string
	method string
}

// Urls API 端点映射
var Urls = map[requestType]requestContent{
	GetVersion: {
		"/api/v1/version",
		"GET",
	},
	GetTargetPkgLists: {
		"/api/v1/package",
		"GET",
	},
	GetCurrentPkgLists: {
		"/api/v1/package",
		"GET",
	},
	GetUpdateLog: {
		"/api/v1/systemupdatelogs",
		"GET",
	},
	GetPkgCVEs: {
		"/api/v1/cve/sync",
		"GET",
	},
	PostProcess: {
		"/api/v1/process",
		"POST",
	},
	PostProcessEvent: {
		"/api/v1/process/events",
		"POST",
	},
	PostResult: {
		"/api/v1/update/status",
		"POST",
	},
}

// tokenMessage 通用响应消息
type tokenMessage struct {
	Result bool            `json:"result"`
	Code   int             `json:"code"`
	Data   json.RawMessage `json:"data"`
}

// tokenErrorMessage 错误响应消息
type tokenErrorMessage struct {
	Result bool   `json:"result"`
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
}

// UpdateTp 更新策略类型
type UpdateTp int

const (
	UnknownUpdate   UpdateTp = 0
	NormalUpdate    UpdateTp = 1 // 更新
	UpdateNow       UpdateTp = 2 // 立即更新 // 以下为强制更新
	UpdateShutdown  UpdateTp = 3 // 关机更新
	UpdateRegularly UpdateTp = 4 // 定时更新
)

// String 返回 UpdateTp 的字符串表示
func (u UpdateTp) String() string {
	switch u {
	case UnknownUpdate:
		return "UnknownUpdate"
	case NormalUpdate:
		return "NormalUpdate"
	case UpdateNow:
		return "UpdateNow"
	case UpdateShutdown:
		return "UpdateShutdown"
	case UpdateRegularly:
		return "UpdateRegularly"
	default:
		return fmt.Sprintf("UpdateTp(%d)", u)
	}
}

// Version 版本信息
type Version struct {
	Version  string `json:"version"`
	Baseline string `json:"baseline"`
	TaskID   int    `json:"taskID"`
}

// Policy 更新策略
type Policy struct {
	Tp   UpdateTp   `json:"tp"`
	Data policyData `json:"data"`
}

type policyData struct {
	UpdateTime string `json:"updateTime"`
}

// repoInfo 仓库信息
type repoInfo struct {
	URI      string `json:"uri"`
	Cdn      string `json:"cdn"`
	CodeName string `json:"codename"`
	Version  string `json:"version"`
	Source   string `json:"source"`
}

// ClientPollSetting 客户端轮询设置
type ClientPollSetting struct {
	CheckPolicyInterval int   `json:"checkPolicyInterval"`
	StartCheckRange     []int `json:"startCheckRange"`
}

// updateMessage 更新消息
type updateMessage struct {
	SystemType        string            `json:"systemType"`
	Version           Version           `json:"version"`
	Policy            Policy            `json:"policy"`
	RepoInfos         []repoInfo        `json:"repoInfos"`
	ClientPollSetting ClientPollSetting `json:"clientPollSetting"`
}

// UpdateLogMeta 更新日志元数据
type UpdateLogMeta struct {
	Baseline      string    `json:"baseline"`
	ShowVersion   string    `json:"showVersion"`
	CnLog         string    `json:"cnLog"`
	EnLog         string    `json:"enLog"`
	LogType       int       `json:"logType"`
	IsUnstable    int       `json:"isUnstable"`
	SystemVersion string    `json:"systemVersion"`
	PublishTime   time.Time `json:"publishTime"`
}

// ShellCheck 检查脚本
type ShellCheck struct {
	Name  string `json:"name"`  // 检查脚本的名字
	Shell string `json:"shell"` // 检查脚本的内容
}

// PackageInfo 软件包信息
type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Need    bool   `json:"need"`
}

// PlatformPackageInfo 平台软件包信息
type PlatformPackageInfo struct {
	Name           string            `json:"name"`
	Need           bool              `json:"need"`
	AllArchVersion []ArchVersionInfo `json:"allArchVersion"`
}

// ArchVersionInfo 架构版本信息
type ArchVersionInfo struct {
	Arch    string `json:"arch"`
	Version string `json:"version"`
}

// packageLists 软件包清单
type packageLists struct {
	Core   []PlatformPackageInfo `json:"core"`   // 必须安装软件包清单
	Select []PlatformPackageInfo `json:"select"` // 可选软件包清单
	Freeze []PlatformPackageInfo `json:"freeze"` // 禁止升级包清单
	Purge  []PlatformPackageInfo `json:"purge"`  // 删除软件包清单
}

// PreInstalledPkgMeta 预装软件包元数据
type PreInstalledPkgMeta struct {
	PreCheck  []ShellCheck `json:"preCheck"`  // 更新前检查脚本
	MidCheck  []ShellCheck `json:"midCheck"`  // 更新后检查脚本
	PostCheck []ShellCheck `json:"postCheck"` // 更新完成重启后检查脚本
	Packages  packageLists `json:"packages"`  // 基线软件包清单
}

// CVEInfo CVE 信息
type CVEInfo struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// CVEMeta CVE 元数据
type CVEMeta struct {
	DataTime string              `json:"dataTime"`
	CVEs     map[string]CVEInfo  `json:"cves"`
	PkgCVEs  map[string][]string `json:"pkgCves"`
}

// ProcessEventType 进程事件类型
type ProcessEventType int

const (
	CheckEnv            ProcessEventType = 1
	GetUpdateEvent      ProcessEventType = 2
	StartDownload       ProcessEventType = 3
	DownloadComplete    ProcessEventType = 4
	StartBackUp         ProcessEventType = 5
	BackUpComplete      ProcessEventType = 6
	StartInstall        ProcessEventType = 7
	MaxProcessEventType ProcessEventType = 8 // Max value marker (actual max is MaxEventType - 1)
)

// String returns string representation of ProcessEventType
func (p ProcessEventType) String() string {
	switch p {
	case CheckEnv:
		return "CheckEnv"
	case GetUpdateEvent:
		return "GetUpdateEvent"
	case StartDownload:
		return "StartDownload"
	case DownloadComplete:
		return "DownloadComplete"
	case StartBackUp:
		return "StartBackUp"
	case BackUpComplete:
		return "BackUpComplete"
	case StartInstall:
		return "StartInstall"
	default:
		return fmt.Sprintf("ProcessEventType(%d)", p)
	}
}

// IsValid checks if ProcessEventType is valid.
func (p ProcessEventType) IsValid() bool {
	return p >= CheckEnv && p < MaxProcessEventType
}

// ProcessEvent process event
type ProcessEvent struct {
	TaskID       int              `json:"taskID"`
	EventType    ProcessEventType `json:"eventType"`
	EventStatus  bool             `json:"eventStatus"` // 是否成功
	EventContent string           `json:"eventContent"`
}

// StatusMessage status message for process reporting
type StatusMessage struct {
	Type           string `json:"type"`           // Message type: info, warning, error
	UpdateType     string `json:"updateType"`     // Update type
	JobDescription string `json:"jobDescription"` // Job description
	Detail         string `json:"detail"`         // Message detail
}

// UpgradeResult 升级结果
type UpgradeResult int8

const (
	UpgradeSucceed UpgradeResult = 0
	UpgradeFailed  UpgradeResult = 1
	CheckFailed    UpgradeResult = 2
)

func (r UpgradeResult) String() string {
	switch r {
	case UpgradeSucceed:
		return "UpgradeSucceed"
	case UpgradeFailed:
		return "UpgradeFailed"
	case CheckFailed:
		return "CheckFailed"
	default:
		return "Unknown"
	}
}

// MsgPostStatus 消息上报状态
type MsgPostStatus string

const (
	NotReady    MsgPostStatus = "not ready"
	WaitPost    MsgPostStatus = "wait post"
	PostSuccess MsgPostStatus = "post success"
	PostFailure MsgPostStatus = "post failure"
)

// UpgradePostMsg 升级上报消息
type UpgradePostMsg struct {
	SerialNumber    string        `json:"serialNumber"`
	MachineID       string        `json:"machineId"`
	UpgradeStatus   UpgradeResult `json:"status"`
	UpgradeErrorMsg string        `json:"msg"`
	TimeStamp       int64         `json:"timestamp"`
	SourceUrl       []string      `json:"sourceUrl"`
	Version         string        `json:"version"`

	PreBuild        string `json:"preBuild"`
	NextShowVersion string `json:"nextShowVersion"`
	PreBaseline     string `json:"preBaseline"`
	NextBaseline    string `json:"nextBaseline"`

	UpgradeStartTime int64 `json:"updateStartAt"`
	UpgradeEndTime   int64 `json:"updateFinishAt"`
	TaskId           int   `json:"taskId"`

	Uuid           string
	PostStatus     MsgPostStatus
	RetryCount     uint32
	upgradeLogPath string
}
