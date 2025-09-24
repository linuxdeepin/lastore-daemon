package cache

import (
	"fmt"
	"os"
	"reflect"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

// UpdateInfo 数据结构体
type RepoInfo struct {
	Name       string   `json:"Name" yaml:"Name"`             // reponame
	Type       string   `json:"Type" yaml:"Type"`             // type : deb , ostree ,rpm ,local
	URL        string   `json:"Url" yaml:"Url"`               // url
	Suite      string   `json:"Suite" yaml:"Suite"`           // suite
	Components []string `json:"Components" yaml:"Components"` // components
	FilePath   string   `json:"FilePath" yaml:"FilePath"`     // file path with local
	HashSha256 string   `json:"HashSha256" yaml:"HashSha256"` // file path with local
}

// merge right to left
func (ts *RepoInfo) Merge(rightRepoInfo RepoInfo) error {

	rightValueList := reflect.ValueOf(rightRepoInfo)
	leftValueList := reflect.ValueOf(ts).Elem()

	for i := 0; i < leftValueList.NumField(); i++ {
		leftField := leftValueList.Field(i)
		rightField := rightValueList.FieldByName(leftValueList.Type().Field(i).Name)

		if rightField.IsValid() && rightField.Interface() != "" && !reflect.DeepEqual(rightField, leftField) {
			//fmt.Printf("rightField: %+v\n", rightField.Interface())
			leftField.Set(rightField)
		}
	}

	return nil
}

// check repo file
func (ts *RepoInfo) CheckRepoFile() error {
	if err := ts.CheckRepoIndexExist(); err != nil {
		return fmt.Errorf("repoinfo filepath err:%+v", err)
	}
	if err := ts.CheckRepoIndexSha256(); err != nil {
		return fmt.Errorf("repoinfo sha256 err:%+v", err)
	}
	return nil
}

// check repo index exist
func (ts *RepoInfo) CheckRepoIndexExist() error {
	if _, err := os.Stat(ts.FilePath); err != nil {
		return err
	}
	return nil
}

// check repo index sha256
func (ts *RepoInfo) CheckRepoIndexSha256() error {

	if err := fs.CheckRepoInfoHashSha256(ts.FilePath, ts.HashSha256); err != nil {
		return fmt.Errorf("check %s err: %+v", ts.FilePath, err)
	}

	return nil
}

// loader repo info to cache info with package list
func (ts *RepoInfo) LoaderPackageInfo(current *CacheInfo) error {
	if err := fs.CheckFileExistState(ts.FilePath); err != nil {
		return fmt.Errorf("file %s not found", ts.FilePath)
	}

	return nil
}
