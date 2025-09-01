package meta

import (
	"encoding/json"
	"fmt"
	"os"
	"io"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
)

func LoadMetaCfg(cfg string, meta *cache.CacheInfo) error {
	// 直接打开文件，避免先检查后读取的竞态条件
	file, err := os.Open(cfg)
	if err != nil {
		return fmt.Errorf("LoadMetaCfg open config failed: %v", err)
	}
	defer file.Close() // 确保文件描述符被关闭

	// 获取文件信息以检查大小
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("LoadMetaCfg stat config failed: %v", err)
	}

	// 限制文件大小，防止内存溢出（10MB 是一个合理的上限）
	const maxSize = 10 * 1024 * 1024 // 10MB
	if fileInfo.Size() > maxSize {
		return fmt.Errorf("LoadMetaCfg config file too large: %d bytes", fileInfo.Size())
	}

	// 读取文件内容
	cfgRaw := make([]byte, fileInfo.Size())
	if _, err := io.ReadFull(file, cfgRaw); err != nil {
		return fmt.Errorf("LoadMetaCfg read config failed: %v", err)
	}

	var updatemeta cache.UpdateInfo
	if err := json.Unmarshal(cfgRaw, &updatemeta); err != nil {
		return fmt.Errorf("LoadMetaCfg parse config failed: %v", err)
	}

	meta.UpdateMetaInfo = updatemeta

	return nil
}
