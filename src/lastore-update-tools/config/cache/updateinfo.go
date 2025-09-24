package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
)

const (
	PreUpdate = iota // pre
	MidCheck         // mid
	PostCheck        // post
	bottomCheck
)

// UpdateInfo 数据结构体
type UpdateInfo struct {
	RepoBackend []RepoInfo `json:"RepoInfo" yaml:"RepoInfo"`                   // repoinfo
	SysCoreList []AppInfo  `json:"CoreList" yaml:"CoreList"`                   // corelist
	PkgDebPath  string     `json:"PkgDebPath" yaml:"PkgDebPath"`               // PkgDebPath
	UUID        string     `json:"UUID" yaml:"UUID"`                           // uuid
	Time        string     `json:"Time" yaml:"Time"`                           // Time
	ApiVersion  string     `json:"ApiVersion" yaml:"ApiVersion" default:"1.0"` // ApiVersion
}

// VerifyUpdateInfo checks if UpdateInfo has valid required fields
func (ts *UpdateInfo) VerifyUpdateInfo() error {
	if len(ts.PkgDebPath) == 0 {
		return fmt.Errorf("pkgDebPath is empty")
	}

	if len(ts.UUID) == 0 {
		return fmt.Errorf("uuid is empty")
	}

	if len(ts.RepoBackend) == 0 {
		return fmt.Errorf("repoBackend is empty")
	}

	// check repo backend list
	for _, repoBackend := range ts.RepoBackend {
		if err := repoBackend.CheckRepoFile(); err != nil {
			return fmt.Errorf("check repo info err: %v", err)
		}
	}

	return nil
}

func (ts *UpdateInfo) LoaderJson(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	cfgRaw, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("LoaderJson read config failed: %v", err)
	}
	if err := json.Unmarshal(cfgRaw, &ts); err != nil {
		return fmt.Errorf("LoaderJson copy config failed: %v", err)
	}
	return nil
}

// RemovedRepoInfo
func (ts *UpdateInfo) RemovedRepoInfo(index int) error {
	repoList := len(ts.RepoBackend)

	// have elements in the repo list that can be removed
	if repoList > 1 && index >= 0 && index < repoList {
		ts.RepoBackend = append(ts.RepoBackend[:index], ts.RepoBackend[index+1:]...)
		return nil
	}

	if repoList < index {
		return errors.New("index out of range")
	}
	if repoList == 1 && index == 0 {
		ts.RepoBackend = nil
		return nil
	}

	return nil
}
