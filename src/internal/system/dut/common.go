// SPDX-FileCopyrightText: 2018 - 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dut

type ErrorCode int

const (
	ChkSuccess            ErrorCode = 0         // 检查成功
	ChkError              ErrorCode = 1 << iota // 检查失败
	ChkInvalidInput                             // 无效参数输入
	ChkFixPkgDependFailed                       // 软件包配置修复失败
	ChkBlockError                               // 阻塞项检查失败
	ChkNonblockError                            // 非阻塞项检查失败
	ChkDynError                                 // 动态检查失败
)

type ExtCode uint

const (
	ChkProgramSuccess    uint = 0         // 检查项成功
	ChkProgramError      uint = 1 << iota // 检查项失败
	ChkMetaInfoFileError                  // 元数据项检查报错
	// precheck阻塞项返回值
	ChkToolsDependError        // 依赖工具项检查报错
	ChkPkgDependError          // precheck、midcheck // 系统存在依赖错误
	ChkCorePkgDependError      // 系统核心包存在依赖错误
	ChkSysDiskOutSpace         // precheck、midcheck(阻塞+非阻塞) 系统盘剩余空间不足
	ChkDataDiskOutSpace        // 数据盘剩余空间不足
	ChkCorePkgNotfound         // precheck、midcheck 系统核心包丢失
	ChkOptionPkgNotfound       // precheck、midcheck 系统可选包丢失
	ChkDpkgVersionNotSupported // DPKG非系统版本
	// midcheck阻塞项返回值
	ChkAptStateError       // 阻塞项，APT安装状态错误
	ChkDpkgStateError      // 阻塞项，DPKG安装状态错误
	ChkPkgListNonexistence // 阻塞项，PkgList清单中的包丢失
	ChkPkgListErrState     // 阻塞项，PkgList清单中的包安装状态错误
	ChkPkgListErrVersion   // 阻塞项，PkgList清单中的包版本错误
	ChkCoreFileMiss        // 阻塞项，系统核心文件丢失
	ChkSysPkgInfoLoadErr   // 阻塞项，当前系统包信息加载失败
	// postcheck阻塞项返回值
	ChkImportantProgressNotRunning // 系统重要进程检查失败

	// midcheck非阻塞项返回值
	ChkCorePkgErrState     // 系统核心包安装状态错误
	ChkCorePkgErrVersion   // 系统核心包版本错误
	ChkOptionPkgErrState   // 系统可选包安装状态错误
	ChkOptionPkgErrVersion // 系统可选包版本错误
	// postcheck非阻塞项返回值
	ChkUuidDirNotExist          // 更新元数据UUID目录不存在
	ChkLogFileNotExist          // 更新日志文件不存在
	ChkLogRmSensitiveInfoFailed // 更新日志脱敏失败
	ChkArchiveFileNotExist      // 更新归档文件不存在
	// dynamic检查
	ChkDynamicScriptErr    // 动态检查失败
	ChkPkgConfigError      // 软件包配置错误
	UpdatePkgInstallFailed // 软件包安装失败
	UpdateRulesCheckFailed // 更新规则检查失败
)

const (
	OptionFirstCheck = "firstCheck"
)
