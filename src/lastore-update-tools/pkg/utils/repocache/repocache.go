// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package repocache

import (
	"encoding/gob"
	"fmt"

	// "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
	"io/ioutil"
	"os"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
	"gopkg.in/yaml.v2"
)

// repocache.gob
type DebRepo struct {
	// reporef = url+suite+component : packages.gz hash
	CacheRef    map[string]string `json:"reporef" yaml:"reporef"`
	AppinfoHash map[string]cache.AppInfo
	// 存储compents 对应的 Packages.gz 的hansh
	// 按仓库 compents : hash
	// 头部是描述信息
	// 结构体中 缓存 struct to 文件的内容地址。uuid 存储
}

// appinfoHash := make(map[string]cache.AppInfo, 100)

func (ts *DebRepo) Dump(save string) error {

	output, err := yaml.Marshal(&ts)
	if err != nil {
		return fmt.Errorf("Dump convert config failed: %v", err)
	}
	err = ioutil.WriteFile(save, output, 0644)
	if err != nil {
		return fmt.Errorf("Dump save cache failed: %v", err)
	}
	return nil
}

func (ts *DebRepo) Loader(cache string) error {
	if _, err := os.Stat(cache); err != nil {
		return err
	}
	cfgRaw, err := ioutil.ReadFile(cache)
	if err != nil {
		return fmt.Errorf("Repo Loader read config failed: %v", err)
	}
	if err := yaml.Unmarshal(cfgRaw, ts); err != nil {
		return fmt.Errorf("Repo Loader copy config failed: %v", err)
	}
	return nil
}

func (ts *DebRepo) LoaderCache(cachePath string) error {
	if _, err := os.Stat(cachePath); err != nil {
		return err
	}

	file, err := os.Open(cachePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var dedata map[string]cache.AppInfo
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&dedata)
	if err != nil {
		return err
	}
	for k, v := range dedata {
		if _, ok := ts.AppinfoHash[k]; ok {
			continue
		}
		ts.AppinfoHash[k] = v
	}
	dedata = nil
	return nil
}

func (ts *DebRepo) LoaderMeta(cache string) error {
	return nil
}

func (ts *DebRepo) SaveMeta(save string) error {
	return nil
}

func (ts *DebRepo) SaveCache(save string, data map[string]cache.AppInfo) error {
	file, err := os.Create(save)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		return err
	}

	return nil
}

func (ts *DebRepo) RepoToCache(cachedir, url, suite, component string) error {

	appinfoHash := make(map[string]cache.AppInfo, 100)

	PackagesInfo := cachedir + "/" + component + "/Packages.gz"
	PackagesHash, err := fs.FileHashSha1(PackagesInfo)
	if err != nil {
		return err
	}

	//log.Debugf("%s:%s", PackagesInfo, PackagesHash)

	// TODO:(heysion) set download repository packages.gz to cache config

	// TODO:(heysion) fix this function
	cmd := fmt.Sprintf(`zcat %s | egrep "^Version:|^Package:|^SHA1:|^SHA256:|^Filename:|^$"`, PackagesInfo)

	outputStream, err := runcmd.RunnerOutput(10, "bash", "-c", cmd)
	if err != nil {
		return err
	}
	//log.Debugf("Read %d", len(outputStream))
	for _, outStream := range strings.Split(outputStream, "\n\n") {
		// log.Debugf("Read %s", outStream)
		appByGz := cache.AppInfo{}
		for _, outLineString := range strings.Split(outStream, "\n") {
			// log.Debugf("Read %s[%d]", outLine, len(outLine))
			if len(outLineString) > 7 {

				splitValue := func(vstr string) string {
					vlist := strings.Split(vstr, ": ")
					// log.Debugf("splitValue:%+v", vlist)
					return vlist[1]
				}

				if strings.HasPrefix(outLineString, "Version:") {
					appByGz.Version = splitValue(outLineString)
				}

				if strings.HasPrefix(outLineString, "Package:") {
					appByGz.Name = splitValue(outLineString)
				}
				if strings.HasPrefix(outLineString, "SHA1:") {
					appByGz.HashSha1 = splitValue(outLineString)
				}
				if strings.HasPrefix(outLineString, "SHA256:") {
					appByGz.HashSha256 = splitValue(outLineString)
				}

				if strings.HasPrefix(outLineString, "Filename:") {
					appByGz.Filename = splitValue(outLineString)
				}

			} else {
				continue
			}

		}
		// log.Debugf("show : %+v", appByGz)

		appByGz.Url = url + "/" + appByGz.Filename

		appinfoHash[fmt.Sprintf("%s#%s", appByGz.Name, appByGz.Version)] = appByGz
		// appWithRepo = append(appWithRepo, appByGz)

	}

	if len(appinfoHash) > 0 {
		ts.SaveCache(cachedir+"/"+component+"/"+PackagesHash+".gob", appinfoHash)
	}
	for k, v := range appinfoHash {
		if _, ok := ts.AppinfoHash[k]; ok {
			continue
		}
		ts.AppinfoHash[k] = v
	}
	//log.Debugf("len:%+v", len(ts.AppinfoHash))

	ts.CacheRef[fmt.Sprintf("%s#%s#%s", url, suite, component)] = PackagesHash
	//log.Debugf("cacheref:%+v", ts.CacheRef)

	appinfoHash = nil

	return nil
}
