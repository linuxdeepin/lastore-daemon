package main

import (
	"flag"
	"fmt"
)

func main() {
	var item = flag.String("item", "", "categories|applications|xcategories")
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
	default:
		flag.Usage()
		return
	}

	if err != nil {
		fmt.Printf("Do %q(%q) failed: %v\n", *item, *fpath, err)
	}
}
