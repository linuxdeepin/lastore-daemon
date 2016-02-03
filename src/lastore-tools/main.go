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
	"flag"
	"fmt"
	"pkg.deepin.io/lib/utils"
)

func main() {
	utils.UnsetEnv("LC_ALL")
	utils.UnsetEnv("LANGUAGE")
	utils.UnsetEnv("LC_MESSAGES")
	utils.UnsetEnv("LANG")

	var item = flag.String("item", "", "categories|applications|xcategories|desktop|lastore-remove|lastore-install|update_infos|mirrors")
	var fpath = flag.String("output", "", "the file to write")

	flag.Parse()
	var err error

	switch *item {
	case "categories":
		err = GenerateCategory(*fpath)
	case "applications":
		err = GenerateApplications(*fpath)
	case "xcategories":
		err = GenerateXCategories(*fpath)
	case "desktop":
		if *fpath == "" {
			err = fmt.Errorf("which directory to save  desktop index files?")
		}
		err = GenerateDesktopIndexes(*fpath)
	case "lastore-remove":
		RemoveAll()
	case "lastore-install":
		InstallAll()

	case "update_infos":
		GenerateUpdateInfos(*fpath)

	case "mirrors":
		err = GenerateMirrors(*fpath)

	default:
		flag.Usage()
		return
	}

	if err != nil {
		fmt.Printf("Do %q(%q) failed: %v\n", *item, *fpath, err)
	}
}
