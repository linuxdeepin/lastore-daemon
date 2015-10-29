package main

import (
	"flag"
	"fmt"
	"strings"
)

func main() {
	var item = flag.String("item", "", "categories|applications|xcategories|desktop")
	var fpath = flag.String("output", "", "the file to write")
	var scanDirs = flag.String("dirs", "/usr/share/applications", "the scan directory when generate desktop index files")

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
		err = GenerateDesktopIndexes(strings.Split(*scanDirs, ","), *fpath)
	default:
		flag.Usage()
		return
	}

	if err != nil {
		fmt.Printf("Do %q(%q) failed: %v\n", *item, *fpath, err)
	}
}
