package update

import (
	"fmt"
	"strings"
	"testing"
)

func TestTODO(t *testing.T) {
	t.Skipf("todo fix")
}

func TestGetNewVersion(t *testing.T) {
	Version := "4:5.57.1.14-1"
	CVersion := "4%3a5.57.1.14-1"
	Version2 := getNewVersion(strings.Index(Version, ":"), "%3a", Version)
	if CVersion != Version2 {
		t.Errorf("failed: %s", Version)
	}
}

func TestGetNewFileNameSw64(t *testing.T) {

	inData1 := struct {
		name    string
		version string
		arch    string
		real    string
	}{"unrar", "1%3a5.6.6-1", "sw%5f64", "unrar_1%3a5.6.6-1_sw%5f64.deb"}
	if rname := getRealFileName(inData1.name, inData1.version, inData1.arch); rname != inData1.real {
		t.Errorf("getRealFileName failed %s", inData1.real)
	} else {
		t.Logf("real name:%s", rname)
	}

	inData2 := struct {
		name    string
		version string
		arch    string
		real    string
	}{"unrar", "1%3a5.6.6-1", "all", "unrar_1%3a5.6.6-1_all.deb"}
	if rname := getRealFileName(inData2.name, inData2.version, ""); rname != inData2.real {
		t.Errorf("getRealFileName failed %s", inData2.real)
	} else {
		t.Logf("real name:%s", rname)
	}

}

func TestGetNewFileName(t *testing.T) {
	oldPath := "/var/lib/apt_aa/libkf5wayland-dev_5.57.0.10-1+eagle_amd64.deb"
	Version := "4:5.57.0.10-1+eagle"

	newPathLeft := "/var/lib/apt_aa/libkf5wayland-dev_4%3a5.57.0.10-1+eagle_amd64.deb"

	Version2 := getNewVersion(strings.Index(Version, ":"), "%3a", Version)

	newPath, err := getNewFileName(Version2, oldPath)
	if err != nil {
		t.Error("getNewFileName failed")
	}

	if newPath != newPathLeft {
		fmt.Printf("%s != %s\n", newPath, newPathLeft)
		t.Error("not compare")
	}

}
