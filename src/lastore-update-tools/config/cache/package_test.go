package cache

import (
	"fmt"
	"reflect"
	"testing"
)

func TestDiff(t *testing.T) {
	coll := []*Software{{Package: "2"}, {Package: "3"}, {Package: "1"}}
	colr := []*Software{{Package: "2"}, {Package: "4"}, {Package: "5"}}
	colll := []*Software{{Package: "2"}, {Package: "3"}, {Package: "1"}, {Package: "4"}, {Package: "6"}}
	colrl := []*Software{{Package: "2"}, {Package: "5"}, {Package: "7"}, {Package: "9"}}

	rl, rr := DiffPackagesSoftware(coll, colr)
	testDiffPrint(rl, rr, []*Software{{Package: "1"}, {Package: "3"}}, []*Software{{Package: "4"}, {Package: "5"}}, t)

	rl, rr = DiffPackagesSoftware(coll, colrl)
	testDiffPrint(rl, rr, []*Software{{Package: "1"}, {Package: "3"}}, []*Software{{Package: "5"}, {Package: "7"}, {Package: "9"}}, t)
	rl, rr = DiffPackagesSoftware(colll, colr)
	testDiffPrint(rl, rr, []*Software{{Package: "1"}, {Package: "3"}, {Package: "6"}}, []*Software{{Package: "5"}}, t)
}

func testDiffPrint(rl, rr, dl, dr []*Software, t *testing.T) {
	if len(rl) != len(dl) {
		t.FailNow()
	}
	if len(rr) != len(dr) {
		t.FailNow()
	}
	mark := true
	for i, item := range rl {
		fmt.Print(item.Package, item.Package == dl[i].Package, " ")
		if item.Package != dl[i].Package {
			mark = false
		}
	}
	fmt.Println()

	for i, item := range rr {
		fmt.Print(item.Package, item.Package == dr[i].Package, " ")
		if item.Package != dr[i].Package {
			mark = false
		}
	}
	fmt.Println()
	if !mark {
		t.FailNow()
	}
}

func TestDependsList(t *testing.T) {
	sf := &Software{Depends: "cockpit-bridge (>= 295-1), cockpit-ws (>= 295-1), cockpit-system (>= 295-1)"}
	dl, err := sf.DependsList()
	if err != nil {
		t.FailNow()
	}
	reflect.DeepEqual(dl, [][2]string{
		{"cockpit-bridge", ">= 295-1"},
		{"cockpit-ws", "295-1"},
		{"cockpit-system", "295-1"},
	})
}
