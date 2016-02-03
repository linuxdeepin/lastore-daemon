/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
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
