package cache

import (
	"testing"
)

func TestCheckOK(t *testing.T) {

	type KV struct {
		k PkgState
		v bool
	}
	dataSet := []KV{
		KV{AppStateDefault, false},
		KV{InstallHalf, false},
		KV{InstallUnpacked, false},
		KV{InstallConfigPending, false},
		KV{InstallTriggerAwait, false},
		KV{InstalledTriggerPending, true},
		KV{InstalledOK, true},
		KV{HoldHalf, false},
		KV{HoldUnpacked, false},
		KV{HoldConfigPending, false},
		KV{HoldTriggerAwait, false},
		KV{HoldTrigerPending, true},
		KV{HoldInstalled, true},
		KV{HoldPurged, true},
		KV{Removed, true},
		KV{RemoveHalf, false},
		KV{Purged, true},
		KV{PurgedHalf, false},
		KV{OnlyConfigFiles, true},

		KV{"", false},
		KV{"r", false},
		KV{"p", false},
		KV{"rx", true},
		KV{"rxxx", false},
		KV{"px", true},
		KV{"pxxx", false},

		KV{"up", false},
		KV{"ux", false},
	}

	for idx, ds := range dataSet {
		if ds.k.CheckOK() != ds.v {
			t.Errorf("CheckOK err [%d] %+v check:%v ", idx, ds, ds.k.CheckOK())
		} else {
			t.Logf("CheckOK %+v ", ds)
		}
	}
}

func TestCheckFailed(t *testing.T) {

	type KV struct {
		k PkgState
		v bool
	}
	dataSet := []KV{
		KV{AppStateDefault, false},
		KV{InstallHalf, true},
		KV{InstallUnpacked, true},
		KV{InstallConfigPending, false},
		KV{InstallTriggerAwait, false},
		KV{InstalledTriggerPending, false},
		KV{InstalledOK, false},
		KV{HoldHalf, true},
		KV{HoldUnpacked, true},
		KV{HoldConfigPending, false},
		KV{HoldTriggerAwait, false},
		KV{HoldTrigerPending, false},
		KV{HoldInstalled, false},
		KV{HoldPurged, false},
		KV{Removed, false},
		KV{RemoveHalf, true},
		KV{Purged, false},
		KV{PurgedHalf, true},

		KV{"", false},
		KV{"r", false},
		KV{"p", false},
		KV{"rx", false},
		KV{"rxxx", false},
		KV{"px", false},
		KV{"pxxx", false},

		KV{"iH", true},
		KV{"up", false},
		KV{"ux", false},
		KV{"uH", false},
		KV{"H", false},
	}

	for idx, ds := range dataSet {
		if dsk, err := ds.k.CheckFailed(); dsk != ds.v {
			t.Errorf("CheckFailed err [%d] %+v check:%v e:%v ", idx, ds, dsk, err)
		} else {
			t.Logf("CheckFailed %+v ", ds)
		}
	}
}

func TestCheckConfigure(t *testing.T) {

	type KV struct {
		k PkgState
		v bool
	}
	dataSet := []KV{
		KV{AppStateDefault, false},
		KV{InstallHalf, false},
		KV{InstallUnpacked, false},
		KV{InstallConfigPending, true},
		KV{InstallTriggerAwait, true},
		KV{InstalledTriggerPending, false},
		KV{InstalledOK, false},
		KV{HoldHalf, false},
		KV{HoldUnpacked, false},
		KV{HoldConfigPending, true},
		KV{HoldTriggerAwait, true},
		KV{HoldTrigerPending, false},
		KV{HoldInstalled, false},
		KV{HoldPurged, false},
		KV{Removed, false},
		KV{RemoveHalf, false},
		KV{Purged, false},
		KV{PurgedHalf, false},

		KV{"", false},
		KV{"r", false},
		KV{"p", false},
		KV{"rx", false},
		KV{"rxxx", false},
		KV{"px", false},
		KV{"pxxx", false},

		KV{"up", false},
		KV{"ux", false},
		KV{"uH", false},
		KV{"H", false},

		KV{"iF", true},
		KV{"iW", true},
		KV{"iw", false},
		KV{"uF", false},
		KV{"xF", false},
	}

	for idx, ds := range dataSet {
		if dsk, err := ds.k.CheckConfigure(); dsk != ds.v {
			t.Errorf("CheckConfigure err [%d] %+v check:%v e:%v ", idx, ds, dsk, err)
		} else {
			t.Logf("CheckConfigure %+v ", ds)
		}
	}
}
