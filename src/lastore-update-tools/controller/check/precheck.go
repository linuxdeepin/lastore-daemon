package check

import (
	"fmt"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"strings"
	// "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/sysinfo"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
)

func CheckVerifyCacheInfo(cfg *cache.CacheInfo) error {
	// check update meta info

	if flags, err := cfg.UpdateMetaInfo.IsEmpty(); flags {
		return fmt.Errorf("%+v not found update meta info", err)
	}
	// check repo backend list
	for _, repoBackend := range cfg.UpdateMetaInfo.RepoBackend {
		if err := repoBackend.CheckRepoFile(); err != nil {
			log.Warnf("repoinfo check err: %v", err)
			return fmt.Errorf("check repo err:%v", err)
		}
	}
	return nil
}

func CheckDPKGVersionSupport(sysCurrPackage map[string]*cache.AppTinyInfo) (int64, error) {
	if dpkgInfo, ok := sysCurrPackage["dpkg"]; ok {
		if !strings.Contains(dpkgInfo.Version, "deepin") && !strings.Contains(dpkgInfo.Version, "dde") {
			log.Debugf("dpkg not support version:%s", dpkgInfo.Version)
			return ecode.CHK_DPKG_VERSION_NOT_SUPPORTED, fmt.Errorf("dpkg not support version:%s", dpkgInfo.Version)
		}
		return ecode.CHK_PROGRAM_SUCCESS, nil
	} else {
		return ecode.CHK_TOOLS_DEPEND_ERROR, fmt.Errorf("dpkg not found in system")
	}
}
