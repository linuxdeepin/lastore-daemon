package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
)

// UpdateInfo 数据结构体
type UpdateInfo struct {
	RepoBackend []RepoInfo   `json:"RepoInfo" yaml:"RepoInfo"`                   // repoinfo
	PkgList     []AppInfo    `json:"PkgList" yaml:"PkgList"`                     // pkglist
	BaseLine    []AppInfo    `json:"BaseLine" yaml:"BaseLine"`                   // baseline
	SysCoreList []AppInfo    `json:"CoreList" yaml:"CoreList"`                   // corelist
	FreezeList  []AppInfo    `json:"FreezeList" yaml:"FreezeList"`               // freezelist
	PurgeList   []AppInfo    `json:"PurgeList" yaml:"PurgeList"`                 // freezelist
	OptionList  []AppInfo    `json:"OptionList" yaml:"OptionList"`               // optionlist
	Rules       []CheckRules `json:"Rules" yaml:"Rules"`                         // rules
	PkgDebPath  string       `json:"PkgDebPath" yaml:"PkgDebPath"`               // PkgDebPath
	UUID        string       `json:"UUID" yaml:"UUID"`                           // uuid
	Time        string       `json:"Time" yaml:"Time"`                           // Time
	ApiVersion  string       `json:"ApiVersion" yaml:"ApiVersion" default:"1.0"` // ApiVersion
}

func (ts *UpdateInfo) VerifyUpdateInfo() error {
	// check update meta info

	// check repo backend list
	for _, repoBackend := range ts.RepoBackend {
		if err := repoBackend.CheckRepoFile(); err != nil {
			log.Warnf("repoinfo check err: %v", err)
			return fmt.Errorf("check repo err:%v", err)
		}
	}
	log.Debugf("repoinfo load ok!")
	return nil
}

func (ts *UpdateInfo) UpdateInfoFormatVerify() error {
	// check update meta info

	// check repo backend list
	for _, repoBackend := range ts.RepoBackend {
		if err := repoBackend.CheckRepoIndexExist(); err != nil {
			log.Warnf("repoinfo check err: %v", err)
			return fmt.Errorf("check repo err:%v", err)
		}
	}
	// check rules format
	for _, ruleVerify := range ts.Rules {
		if _, err := ruleVerify.IsEmpty(); err != nil {
			log.Warnf("rules format check err:%v", err)
			return fmt.Errorf("rules format check err:%v", err)
		}
	}
	log.Debugf("update info format verify")
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
				//log.Debugf("merge key %v", key)
			}

		}
	}

	OtherMerge([]string{
		"RepoBackend",
		"BaseLine", "SysCoreList",
		"FreezeList", "PurgeList",
		"OptionList", "Rules",
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

// check update is empty
func (ts *UpdateInfo) IsEmpty() (bool, error) {

	if len(ts.PkgList) == 0 {
		log.Debugf("PkgList is empty")
		return true, fmt.Errorf("PkgList")
	}

	if len(ts.PkgDebPath) == 0 {
		log.Debugf("PkgDebPath is empty")
		return true, fmt.Errorf("PkgDebPath")
	}

	if len(ts.UUID) == 0 {
		log.Debugf("UUID is empty")
		return true, fmt.Errorf("UUID")
	}

	// FIXME(heysion) 需要baseline的检查
	if len(ts.BaseLine) == 0 {
		log.Warnf("Base line check empty")
	}

	if len(ts.SysCoreList) == 0 {
		log.Warnf("Core list check empty")
	}

	if len(ts.RepoBackend) == 0 {
		log.Debugf("RepoBackend is empty")
		return true, fmt.Errorf("RepoBackend")
	}

	return false, nil
}
