package main

import (
	"fmt"
	"net/http"
)

// Touch send a request to server for recording package
func Touch(arch, region string, packages ...string) {
	for _, pkg := range packages {
		url := fmt.Sprintf("http://download.lastore.deepin.org/get/%s/%s?&f=%s",
			arch,
			pkg,
			region,
		)
		resp, err := http.Get(url)
		if err != nil {
			return
		}
		resp.Body.Close()
	}
}
