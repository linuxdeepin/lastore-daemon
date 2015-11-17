package main

import (
	"encoding/json"
	"fmt"
	"internal/system"
	"net/http"
)

var ServerAPI = "http://api.lastore.deepin.org"

func GenerateMirrors(fpath string) error {
	ms, err := LoadMirrorSources(ServerAPI)
	if err != nil {
		return err
	}
	return writeData(fpath, ms)
}

// LoadMirrorSources return supported MirrorSource from remote server
func LoadMirrorSources(server string) ([]system.MirrorSource, error) {
	rep, err := http.Get(server + "/mirrors")
	if err != nil {
		return nil, err
	}
	defer rep.Body.Close()

	d := json.NewDecoder(rep.Body)
	var v struct {
		StatusCode    int    `json:"status_code"`
		StatusMessage string `json:"status_message"`
		Data          []struct {
			Id       string                       `json:"id"`
			Weight   int                          `json:"weight"`
			Name     string                       `json:"name"`
			Url      string                       `json:"url"`
			Location string                       `json:"location"`
			Locale   map[string]map[string]string `json:"locale"`
		} `json:"data"`
	}
	err = d.Decode(&v)
	if err != nil {
		return nil, err
	}

	if v.StatusCode != 0 {
		return nil, fmt.Errorf("LoadMirrorSources: featch(%q) error: %q",
			server+"/mirrors", v.StatusMessage)
	}

	var r []system.MirrorSource
	for _, raw := range v.Data {
		s := system.MirrorSource{
			Id:         raw.Id,
			Name:       raw.Name,
			Url:        raw.Url,
			NameLocale: make(map[string]string),
		}
		for k, v := range raw.Locale {
			s.NameLocale[k] = v["name"]
		}
		r = append(r, s)
	}
	return r, nil
}
