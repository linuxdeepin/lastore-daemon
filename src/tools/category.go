package main

import (
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"io/ioutil"
	"net/http"
)

const appstoreURI = "http://appstore.api.deepin.test"
const lastoreURI = "http://repository.api.deepin.test"

func decodeData(url string, data interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		log.Infof("can't get %q \n", url)
		return nil
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	var wrap struct {
		StatusCode    int         `json:"status_code"`
		StatusMessage string      `json:"status_message"`
		Data          interface{} `json:"data"`
	}
	wrap.Data = data

	err = d.Decode(&wrap)
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
	err := decodeData(url, &d)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	return writeData(fpath, d)

}

type AppInfo struct {
	Id         string            `json:"id"`
	Category   string            `json:"category"`
	Name       string            `json:"name"`
	NameLocale map[string]string `json:"name_locale"`
}

func GenerateApplications(fpath string) error {
	appsUrl := appstoreURI + "/applications"
	var apps []struct {
		Id       string `json:"id"`
		Category string `json:"category"`
	}

	err := decodeData(appsUrl, &apps)
	if err != nil {
		return err
	}

	var infos = make(map[string]AppInfo)

	for _, app := range apps {
		metaUrl := lastoreURI + "/metadata/" + app.Id
		var v struct {
			Name    string
			Locales map[string]map[string]string
		}
		decodeData(metaUrl, &v)
		info := AppInfo{
			Id:         app.Id,
			Category:   app.Category,
			Name:       v.Name,
			NameLocale: make(map[string]string),
		}
		for lang, data := range v.Locales {
			if name, ok := data["name"]; ok {
				info.NameLocale[lang] = name
			}
		}
		infos[info.Id] = info
	}

	fmt.Println("XXX:", len(infos))
	return writeData(fpath, infos)
}

func GenerateXCategories(fpath string) error {
	var data = make(map[string]string)
	for old, deepin := range xCategoryNameIdMap {
		data[old] = deepin
	}
	for old, deepin := range extraXCategoryNameIdMap {
		data[old] = deepin
	}
	return writeData(fpath, data)
}
