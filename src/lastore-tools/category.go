package main

import (
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"io/ioutil"
	"net/http"
)

const appstoreURI = "http://api.appstore.deepin.org"
const lastoreURI = "http://api.lastore.deepin.test"

func decodeData(wrap bool, url string, data interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		log.Infof("can't get %q \n", url)
		return nil
	}
	defer resp.Body.Close()
	d := json.NewDecoder(resp.Body)

	if wrap {
		var wrap struct {
			StatusCode    int         `json:"status_code"`
			StatusMessage string      `json:"status_message"`
			Data          interface{} `json:"data"`
		}
		wrap.Data = data
		err = d.Decode(&wrap)
	} else {
		err = d.Decode(&data)
	}

	if err != nil {
		return err
	}
	return nil
}

func writeData(fpath string, data interface{}) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fpath, content, 0644)
}

func GenerateCategory(fpath string) error {
	url := appstoreURI + "/" + "categories"

	var d interface{}
	err := decodeData(true, url, &d)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	return writeData(fpath, d)

}

type AppInfo struct {
	Id         string            `json:"id"`
	Category   string            `json:"category"`
	Name       string            `json:"name"`
	LocaleName map[string]string `json:"locale_name"`
}

func GenerateApplications(fpath string) error {
	apps := make(map[string]AppInfo)
	err := decodeData(false, "http://api.appstore.deepin.org/info/all", &apps)
	apps["deepin-appstore"] = AppInfo{
		Id:       "deepin-appstore",
		Category: "system",
		Name:     "deepin store",
		LocaleName: map[string]string{
			"zh_CN": "深度商店",
		},
	}
	if err != nil {
		return err
	}
	return writeData(fpath, apps)
}
