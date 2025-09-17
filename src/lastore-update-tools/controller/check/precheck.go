package check

import (
	"fmt"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
)

func CheckVerifyCacheInfo(cfg *cache.CacheInfo) error {
	// check update meta info

	// check repo backend list
	for _, repoBackend := range cfg.UpdateMetaInfo.RepoBackend {
		if err := repoBackend.CheckRepoFile(); err != nil {
			logger.Warningf("repoinfo check err: %v", err)
			return fmt.Errorf("check repo err:%v", err)
		}
	}
	return nil
}

func CheckDPKGVersionSupport(sysCurrPackage map[string]*cache.AppTinyInfo) (int64, error) {
	if dpkgInfo, ok := sysCurrPackage["dpkg"]; ok {
		if !strings.Contains(dpkgInfo.Version, "deepin") && !strings.Contains(dpkgInfo.Version, "dde") {
			logger.Debugf("dpkg not support version:%s", dpkgInfo.Version)
			return ecode.CHK_DPKG_VERSION_NOT_SUPPORTED, fmt.Errorf("dpkg not support version:%s", dpkgInfo.Version)
		}
		return ecode.CHK_PROGRAM_SUCCESS, nil
	} else {
		return ecode.CHK_TOOLS_DEPEND_ERROR, fmt.Errorf("dpkg not found in system")
	}
}
