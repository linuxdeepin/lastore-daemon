package system

import (
	"encoding/json"
	"os"
)

type MirrorSource struct {
	Id   string
	Name string
	Url  string

	NameLocale map[string]string
}

var DefaultMirror = MirrorSource{
	Id:   "default",
	Url:  "http://cdn.packages.deepin.com/deepin",
	Name: "default",
}

func DecodeJson(fpath string, d interface{}) error {
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewDecoder(f).Decode(&d)
}

func EncodeJson(fpath string, d interface{}) error {
	f, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(d)
}
