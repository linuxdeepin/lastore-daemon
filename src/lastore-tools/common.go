package main

import (
	"strings"

	"github.com/linuxdeepin/go-lib/keyfile"
)

const (
	aptConfDir           = "/etc/apt/apt.conf.d"
	tokenConfFileName    = "99lastore-token.conf" // #nosec G101
	securityConfFileName = "99security.conf"
	osVersion            = "/etc/os-version"
	baseline             = "/etc/os-baseline"
)

// 1030版本的token肯定和当前的不一样，是否按照新版本的token规则重新获取并保存，还是直接用旧版的token 发送
// 更新 99lastore-token.conf 文件的内容
func genToken() string {
	logger.Debug("start updateTokenConfigFile")
	systemInfo := getSystemInfo()
	// tokenPath := path.Join(aptConfDir, tokenConfFileName)
	var tokenSlice []string
	tokenSlice = append(tokenSlice, "a="+systemInfo.SystemName)
	tokenSlice = append(tokenSlice, "b="+systemInfo.ProductType)
	tokenSlice = append(tokenSlice, "c="+systemInfo.EditionName)
	tokenSlice = append(tokenSlice, "v="+systemInfo.Version)
	tokenSlice = append(tokenSlice, "i="+systemInfo.HardwareId)
	tokenSlice = append(tokenSlice, "m="+systemInfo.Processor)
	tokenSlice = append(tokenSlice, "ac="+systemInfo.Arch)
	tokenSlice = append(tokenSlice, "cu="+systemInfo.Custom)
	tokenSlice = append(tokenSlice, "sn="+systemInfo.SN)
	tokenSlice = append(tokenSlice, "vs="+systemInfo.HardwareVersion)
	tokenSlice = append(tokenSlice, "oid="+systemInfo.OEMID)
	tokenSlice = append(tokenSlice, "pid="+systemInfo.ProjectId)
	tokenSlice = append(tokenSlice, "baseline="+systemInfo.Baseline)
	tokenSlice = append(tokenSlice, "st="+systemInfo.SystemType)
	token := strings.Join(tokenSlice, ";")
	token = strings.Replace(token, "\n", "", -1)

	// 按照新的token规则组装，在旧版系统中是否需要保存到本地
	// tokenContent := []byte("Acquire::SmartMirrors::Token \"" + token + "\";\n")
	// err := ioutil.WriteFile(tokenPath, tokenContent, 0644) // #nosec G306
	// if err != nil {
	// 	logger.Warning(err)
	// }
	// TODO: 使用教育版token，获取仓库
	// if logger.GetLogLevel() == log.LevelDebug {
	// 	token = "a=edu-20-std;b=Desktop;c=E;v=20.1060.11068.101.100;i=905923cfb835f3649e79fa90b28dad6fa973425a12d1b6a2ebd3dcf4a52eab92;m=Hygon C86 3250 8-core Processor;ac=amd64;cu=0;sn=N9DA5MAAAFPSL66NBNEAAVS5G;vs=Dhyana+;oid=f1800c30-ceb6-58a4-bcb2-0e4a565947a6;pid=;baseline=edu-20-std-0002;st="
	// }
	return token
}

func getGeneralValueFromKeyFile(path, key string) string {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(path)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	content, err := kf.GetString("General", key)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return content
}

func getCurrentBaseline() string {
	return getGeneralValueFromKeyFile(baseline, "Baseline")
}

func getCurrentSystemType() string {
	return getGeneralValueFromKeyFile(baseline, "SystemType")
}
