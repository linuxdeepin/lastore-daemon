package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
)

// UpdateInfo 数据结构体
type UpdateInfo struct {
	RepoBackend []RepoInfo   `json:"RepoInfo" yaml:"RepoInfo"`                   // repoinfo
	PkgList     []AppInfo    `json:"PkgList" yaml:"PkgList"`                     // pkglist
	SysCoreList []AppInfo    `json:"CoreList" yaml:"CoreList"`                   // corelist
	Rules       []CheckRules `json:"Rules" yaml:"Rules"`                         // rules
	PkgDebPath  string       `json:"PkgDebPath" yaml:"PkgDebPath"`               // PkgDebPath
	UUID        string       `json:"UUID" yaml:"UUID"`                           // uuid
	Time        string       `json:"Time" yaml:"Time"`                           // Time
	ApiVersion  string       `json:"ApiVersion" yaml:"ApiVersion" default:"1.0"` // ApiVersion
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

	// check rules format
	for _, ruleVerify := range ts.Rules {
		if _, err := ruleVerify.IsEmpty(); err != nil {
			return fmt.Errorf("rules format check err: %v", err)
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

func (ts *UpdateInfo) MergeConfig(newUpdateInfo UpdateInfo) error {

	if ts.UUID != newUpdateInfo.UUID {
		return fmt.Errorf("UUID mismatch")
	}

	// merge pkg list
	for idx, newpkginfo := range newUpdateInfo.PkgList {
		if newpkginfo.HashSha256 != "" {
			continue
		}
		archIdex := strings.Index(newpkginfo.Name, ":")
		if archIdex > 0 {
			newUpdateInfo.PkgList[idx].Name = newpkginfo.Name[:archIdex]
			newUpdateInfo.PkgList[idx].Arch = strings.TrimSpace(newpkginfo.Name[archIdex+1:])
			newpkginfo.Name = newUpdateInfo.PkgList[idx].Name
			newpkginfo.Arch = newUpdateInfo.PkgList[idx].Arch
		}
		for _, oldpkginfo := range ts.PkgList {
			if oldpkginfo.HashSha256 == "" {
				break
			}
			if oldpkginfo.Name == newpkginfo.Name && oldpkginfo.Version == newpkginfo.Version {
				if oldpkginfo.Arch == newpkginfo.Arch {
					newUpdateInfo.PkgList[idx] = oldpkginfo
					break
				}
				if oldpkginfo.Arch == sysRealArch && newpkginfo.Arch == "" {
					newUpdateInfo.PkgList[idx] = oldpkginfo
					break
				}
				if oldpkginfo.Arch == "all" && (newpkginfo.Arch == "" || newpkginfo.Arch == sysRealArch) {
					newUpdateInfo.PkgList[idx] = oldpkginfo
					break
				}
			}
		}
	}
	ts.PkgList = newUpdateInfo.PkgList

	// other values
	OtherMerge := func(keylist []string) {
		destValue := reflect.ValueOf(ts).Elem()
		srcValue := reflect.ValueOf(&newUpdateInfo).Elem()
		for _, key := range keylist {
			destField := destValue.FieldByName(key)
			srcField := srcValue.FieldByName(key)
			if srcField.IsValid() && srcField.Interface() != "" && !reflect.DeepEqual(srcField, destField) {
				destField.Set(srcField)
				//logger.Debugf("merge key %v", key)
			}

		}
	}

	OtherMerge([]string{
		"RepoBackend",
		"SysCoreList",
		"Rules",
		"PkgDebPath", "Time"})

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
