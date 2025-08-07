package meta

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
)

func LoadMetaCfg(cfg string, meta *cache.CacheInfo) error {

	if _, err := os.Stat(cfg); err != nil {
		return err
	}
	cfgRaw, err := ioutil.ReadFile(cfg)

	if err != nil {
		return fmt.Errorf("LoadMetaCfg read config failed: %v", err)
	}

	var updatemeta cache.UpdateInfo
	if err := json.Unmarshal(cfgRaw, &updatemeta); err != nil {
		return fmt.Errorf("LoadMetaCfg copy config failed: %v", err)
	}

	meta.UpdateMetaInfo = updatemeta

	return nil
}
