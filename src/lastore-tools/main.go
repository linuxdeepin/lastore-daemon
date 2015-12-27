package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	os.Unsetenv("LC_ALL")
	os.Unsetenv("LANGUAGE")
	os.Unsetenv("LC_MESSAGES")
	os.Unsetenv("LANG")

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
