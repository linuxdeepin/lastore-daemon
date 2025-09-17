// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package ecode

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type RetMsg struct {
	Code    int64    `json:"Code" yaml:"Code" default:"0"`
	Msg     []string `json:"Msg" yaml:"Msg" default:""`
	Ext     *RetMsg  `json:"Ext,omitempty" yaml:"Msg,omitempty"`
	LogPath []string `json:"LogPath,omitempty" yaml:"LogPath,omitempty"`
}

// 返回值及信息
const (
	CHK_SUCCESS int64 = 0
	CHK_ERROR   int64 = 1 << iota
	CHK_INVALID_INPUT
	PKG_FIX_TOOLS_FAILED // unused for return
	CHK_BLOCK_ERROR
	CHK_NONBLOCK_ERROR
	CHK_DYN_ERROR
)

const (
	//程序成功
	CHK_PROGRAM_SUCCESS int64 = 0
	//程序错误
	CHK_PROGRAM_ERROR int64 = 1 << iota //TODO:(DingHao)新增，要将所有CHK_ERROR替换成CHK_PROGRAM_ERROR
	CHK_METAINFO_FILE_ERROR
	//precheck阻塞项返回值
	CHK_TOOLS_DEPEND_ERROR
	CHK_PKG_DEPEND_ERROR //precheck、midcheck
	CHK_CORE_PKG_DEPEND_ERROR
	CHK_SYS_DISK_OUT_SPACE //precheck、midcheck(阻塞+非阻塞)
	CHK_DATA_DISK_OUT_SPACE
	CHK_CORE_PKG_NOTFOUND   //precheck、midcheck
	CHK_OPTION_PKG_NOTFOUND //precheck、midcheck
	CHK_DPKG_VERSION_NOT_SUPPORTED
	//midcheck阻塞项返回值
	CHK_APT_STATE_ERROR     //阻塞项，APT状态错误
	CHK_DPKG_STATE_ERROR    //阻塞项，DPKG状态错误
	CHK_PKGLIST_INEXISTENCE //阻塞项，pkglist中的包丢失
	CHK_PKGLIST_ERR_STATE   //阻塞项，pkglist中的包状态错误
	CHK_PKGLIST_ERR_VERSION //阻塞项，pkglist中的包版本错误
	CHK_CORE_PKG_ERR_STATE
	CHK_CORE_PKG_ERR_VERSION
	CHK_CORE_FILE_MISS          //阻塞项，系统核心文件丢失
	CHK_SYS_PKG_INFO_LOAD_ERROR //阻塞项，系统包信息读取失败
	//postcheck阻塞项返回值
	CHK_IMPORTANT_PROGRESS_NOT_RUNNING

	//midcheck非阻塞项返回值
	CHK_OPTION_PKG_ERR_STATE
	CHK_OPTION_PKG_ERR_VERSION
	//postcheck非阻塞项返回值
	CHK_UUID_DIR_NOT_EXIST
	CHK_LOG_FILE_NOT_EXIST
	CHK_LOG_RM_SENSITIVE_INFO_FAILED
	CHK_ARCHIVE_FILE_NOT_EXIST

	CHK_DYNAMIC_SCRIPT_ERROR // dynamic检查错误
	PKG_CHK_CONFIG_ERROR     // check package configure found error
	PKG_CHK_DEPEND_ERROR     // check package found error
	PKG_FIX_FAILED           // fix package failed error
	// update
	UPDATE_PKG_INSTALL_FAILED
	UPDATE_PKG_PURGE_FAILED
	UPDATE_RULES_CHECK_FAILED
	CLEAR_UPDATE_CACHE_FAILED
	SYS_INTERNAL_ERROR // 系统内部错误
)

var ErrorCodeMapping = map[int64]string{
	CHK_SUCCESS:          "SUCCESS",
	CHK_ERROR:            "ERROR",
	CHK_INVALID_INPUT:    "CHK_INVALID_INPUT",
	PKG_FIX_TOOLS_FAILED: "PKG_FIX_TOOLS_FAILED",
	CHK_BLOCK_ERROR:      "CHK_BLOCK_ERROR",
	CHK_NONBLOCK_ERROR:   "CHK_NONBLOCK_ERROR",
	CHK_DYN_ERROR:        "CHK_DYN_ERROR",
}

var ErrorCodeMappingCN = map[int64]string{
	CHK_SUCCESS:          "成功",
	CHK_ERROR:            "失败",
	CHK_INVALID_INPUT:    "无效参数输入",
	PKG_FIX_TOOLS_FAILED: "修复检查工具失败",
	CHK_BLOCK_ERROR:      "阻塞项检查失败",
	CHK_NONBLOCK_ERROR:   "非阻塞项检查失败",
	CHK_DYN_ERROR:        "动态检查失败",
}

var ExtCodeMapping = map[int64]string{
	CHK_PROGRAM_SUCCESS:            "CHK_PROGRAM_SUCCESS",
	CHK_PROGRAM_ERROR:              "CHK_PROGRAM_ERROR",
	CHK_METAINFO_FILE_ERROR:        "CHK_METAINFO_FILE_ERROR",
	CHK_TOOLS_DEPEND_ERROR:         "CHK_TOOLS_DEPEND_ERROR",
	CHK_PKG_DEPEND_ERROR:           "CHK_PKG_DEPEND_ERROR",
	CHK_CORE_PKG_DEPEND_ERROR:      "CHK_CORE_PKG_DEPEND_ERROR",
	CHK_SYS_DISK_OUT_SPACE:         "CHK_SYS_DISK_OUT_SPACE",
	CHK_DATA_DISK_OUT_SPACE:        "CHK_DATA_DISK_OUT_SPACE",
	CHK_CORE_PKG_NOTFOUND:          "CHK_CORE_PKG_NOTFOUND",
	CHK_OPTION_PKG_NOTFOUND:        "CHK_OPTION_PKG_NOTFOUND",
	CHK_DPKG_VERSION_NOT_SUPPORTED: "CHK_DPKG_VERSION_NOT_SUPPORTED",

	CHK_APT_STATE_ERROR:         "CHK_APT_STATE_ERROR",
	CHK_DPKG_STATE_ERROR:        "CHK_DPKG_STATE_ERROR",
	CHK_PKGLIST_INEXISTENCE:     "CHK_PKGLIST_INEXISTENCE",
	CHK_PKGLIST_ERR_STATE:       "CHK_PKGLIST_ERR_STATE",
	CHK_PKGLIST_ERR_VERSION:     "CHK_PKGLIST_ERR_VERSION",
	CHK_CORE_FILE_MISS:          "CHK_CORE_FILE_MISS",
	CHK_SYS_PKG_INFO_LOAD_ERROR: "CHK_SYS_PKG_INFO_LOAD_ERROR",

	CHK_IMPORTANT_PROGRESS_NOT_RUNNING: "CHK_IMPORTANT_PROGRESS_NOT_RUNNING",

	CHK_CORE_PKG_ERR_STATE:     "CHK_CORE_PKG_ERR_STATE",
	CHK_CORE_PKG_ERR_VERSION:   "CHK_CORE_PKG_ERR_VERSION",
	CHK_OPTION_PKG_ERR_STATE:   "CHK_OPTION_PKG_ERR_STATE",
	CHK_OPTION_PKG_ERR_VERSION: "CHK_OPTION_PKG_ERR_VERSION",

	CHK_UUID_DIR_NOT_EXIST:           "CHK_UUID_DIR_NOT_EXIST",
	CHK_LOG_FILE_NOT_EXIST:           "CHK_LOG_FILE_NOT_EXIST",
	CHK_LOG_RM_SENSITIVE_INFO_FAILED: "CHK_LOG_RM_SENSITIVE_INFO_FAILED",
	CHK_ARCHIVE_FILE_NOT_EXIST:       "CHK_ARCHIVE_FILE_NOT_EXIST",
	CHK_DYNAMIC_SCRIPT_ERROR:         "CHK_DYNAMIC_SCRIPT_ERROR",
	PKG_CHK_CONFIG_ERROR:             "PKG_CHK_CONFIG_ERROR",
	PKG_CHK_DEPEND_ERROR:             "PKG_CHK_DEPEND_ERROR",
	PKG_FIX_FAILED:                   "PKG_FIX_FAILED",
	UPDATE_PKG_INSTALL_FAILED:        "UPDATE_PACKAGE_INSTALL_FAILED",
	UPDATE_PKG_PURGE_FAILED:          "UPDATE_PKG_PURGE_FAILED",
	UPDATE_RULES_CHECK_FAILED:        "UPDATE_RULES_CHECK_FAILED",
	CLEAR_UPDATE_CACHE_FAILED:        "CLEAR_UPDATE_CACHE_FAILED",
	SYS_INTERNAL_ERROR:               "SYS_INTERNAL_ERROR",
}

var ExtCodeMappingCN = map[int64]string{
	CHK_PROGRAM_SUCCESS:            "检查项成功",
	CHK_PROGRAM_ERROR:              "检查项失败",
	CHK_METAINFO_FILE_ERROR:        "元数据项检查报错",
	CHK_TOOLS_DEPEND_ERROR:         "依赖工具项检查报错",
	CHK_PKG_DEPEND_ERROR:           "系统存在依赖错误",
	CHK_CORE_PKG_DEPEND_ERROR:      "系统核心包存在依赖错误",
	CHK_SYS_DISK_OUT_SPACE:         "系统盘剩余空间不足",
	CHK_DATA_DISK_OUT_SPACE:        "数据盘剩余空间不足",
	CHK_CORE_PKG_NOTFOUND:          "系统核心包丢失",
	CHK_OPTION_PKG_NOTFOUND:        "系统可选包丢失",
	CHK_DPKG_VERSION_NOT_SUPPORTED: "DPKG非系统版本",

	CHK_APT_STATE_ERROR:         "APT安装状态错误",
	CHK_DPKG_STATE_ERROR:        "DPKG安装状态错误",
	CHK_PKGLIST_INEXISTENCE:     "待安装清单中的包丢失",
	CHK_PKGLIST_ERR_STATE:       "待安装清单中的包安装状态错误",
	CHK_PKGLIST_ERR_VERSION:     "待安装清单中的包版本错误",
	CHK_CORE_FILE_MISS:          "系统核心文件丢失",
	CHK_SYS_PKG_INFO_LOAD_ERROR: "当前系统包信息加载失败",

	CHK_IMPORTANT_PROGRESS_NOT_RUNNING: "系统重要进程检查失败",

	CHK_CORE_PKG_ERR_STATE:     "系统核心包安装状态错误",
	CHK_CORE_PKG_ERR_VERSION:   "系统核心包版本错误",
	CHK_OPTION_PKG_ERR_STATE:   "系统可选包安装状态错误",
	CHK_OPTION_PKG_ERR_VERSION: "系统可选包版本错误",

	CHK_UUID_DIR_NOT_EXIST:           "更新元数据UUID目录不存在",
	CHK_LOG_FILE_NOT_EXIST:           "更新日志文件不存在",
	CHK_LOG_RM_SENSITIVE_INFO_FAILED: "更新日志脱敏失败",
	CHK_ARCHIVE_FILE_NOT_EXIST:       "更新归档文件不存在",
	CHK_DYNAMIC_SCRIPT_ERROR:         "动态检查失败",
	PKG_CHK_CONFIG_ERROR:             "存在配置错误",
	PKG_CHK_DEPEND_ERROR:             "存在依赖错误",
	PKG_FIX_FAILED:                   "修复失败",
	UPDATE_PKG_INSTALL_FAILED:        "软件包安装失败",
	UPDATE_PKG_PURGE_FAILED:          "软件包卸载失败",
	UPDATE_RULES_CHECK_FAILED:        "更新规则检查失败",
	CLEAR_UPDATE_CACHE_FAILED:        "清理更新过程数据失败",
	SYS_INTERNAL_ERROR:               "内部错误",
}

func (p *RetMsg) SetError(code, ext int64) {
	if code != 0 && ext != 0 {
		p.Push(code)
		p.PushExt(ext)
	}
}

func (p *RetMsg) SetErrorExtMsg(code, ext int64, msg string) {
	if code != 0 && ext != 0 {
		//log.Debugf("code：%v ext:%v msg:%v", code, ext, msg)
		p.Push(code)
		p.PushExtCodeMsg(ext, msg)
	}
}

func (p *RetMsg) PushExtMsg(s string) {
	if p.Ext == nil {
		p.Ext = &RetMsg{}
	}
	if s != "" {
		p.Ext.Msg = append(p.Ext.Msg, s)
	}

}

func (p *RetMsg) ExitOutput(code, ext int64, msg string) {
	p.SetErrorAndOutput(code, ext, msg, true)
}

func (p *RetMsg) SetErrorAndOutput(code, ext int64, msg string, exit bool) {
	if code != 0 && ext != 0 {
		p.Push(code)
		p.PushExtCodeMsg(ext, msg)
	}
	p.RetMsgToJson()
	if p.Code != 0 && exit {
		os.Exit(int(p.Code))
	}
}

// TODO:(DingHao)需要传入参数，暂定无返回值
func (p *RetMsg) Push(retcode int64) {
	curMessage := ""
	if strings.HasPrefix(os.Getenv("LANG"), "zh_CN") {
		curMessage = ErrorCodeMappingCN[retcode]
	} else {
		curMessage = ErrorCodeMapping[retcode]
	}

	if (p.Code & retcode) == 0 {
		p.Code |= retcode
		p.Msg = append([]string{curMessage}, p.Msg...)
	}

}

func (p *RetMsg) PushExt(retcode int64) {
	if p.Ext == nil {
		p.Ext = &RetMsg{}
	}

	curMessage := ""
	if strings.HasPrefix(os.Getenv("LANG"), "zh_CN") {
		curMessage = ExtCodeMappingCN[retcode]
	} else {
		curMessage = ExtCodeMapping[retcode]
	}

	if (p.Ext.Code & retcode) == 0 {
		p.Ext.Code |= retcode
		p.Ext.Msg = append([]string{curMessage}, p.Ext.Msg...)
	}

}

func (p *RetMsg) PushExtCodeMsg(retcode int64, msg string) {
	if p.Ext == nil {
		p.Ext = &RetMsg{}
	}

	curMessage := ""
	if strings.HasPrefix(os.Getenv("LANG"), "zh_CN") {
		curMessage = ExtCodeMappingCN[retcode]
	} else {
		curMessage = ExtCodeMapping[retcode]
	}

	if (p.Ext.Code & retcode) == 0 {
		p.Ext.Code |= retcode
		p.Ext.Msg = append([]string{curMessage}, p.Ext.Msg...)
	}
	if msg != "" {
		p.Ext.Msg = append(p.Ext.Msg, msg)
	}

}

// render return message to json
func (ts *RetMsg) RetMsgToJson() {
	output, err := json.Marshal(ts)

	if err != nil {
		fmt.Printf("\n{Code:-1,Msg:%s}\n", err)
		os.Exit(-1)
	}
	fmt.Fprintf(os.Stderr, "%s\n", output)

}

// ToJson return json string
func (ts *RetMsg) ToJson() (string, error) {
	output, err := json.Marshal(ts)
	if err != nil {
		return "", err
	}
	return string(output), nil
}
