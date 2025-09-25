package check

import (
	"fmt"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
)

func CheckVerifyCacheInfo(cfg *cache.CacheInfo) error {
	// validate update meta info
	if err := cfg.UpdateMetaInfo.VerifyUpdateInfo(); err != nil {
		return fmt.Errorf("update meta info verify err: %v", err)
	}

	return nil
}

func CheckDPKGVersionSupport(sysCurrPackage map[string]*cache.AppTinyInfo) error {
	if dpkgInfo, ok := sysCurrPackage["dpkg"]; ok {
		if !strings.Contains(dpkgInfo.Version, "deepin") && !strings.Contains(dpkgInfo.Version, "dde") {
			logger.Debugf("dpkg not support version:%s", dpkgInfo.Version)
			return &system.JobError{
				ErrType:      system.ErrorDpkgVersion,
				ErrDetail:    fmt.Sprintf("dpkg not support version:%s", dpkgInfo.Version),
				IsCheckError: true,
			}
		}
		return nil
	} else {
		return &system.JobError{
			ErrType:      system.ErrorDpkgNotFound,
			ErrDetail:    fmt.Sprintf("dpkg not found in system"),
			IsCheckError: true,
		}
	}
}
