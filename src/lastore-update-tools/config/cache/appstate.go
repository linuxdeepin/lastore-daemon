package cache

import (
	"fmt"
)

type PkgState string

const (
	// Failed
	AppStateDefault PkgState = ""
	// Install Failed
	InstallHalf     PkgState = "iH" // iH
	InstallUnpacked PkgState = "iU" // iU

	// Install configure failed
	InstallConfigPending PkgState = "iF" // iF
	InstallTriggerAwait  PkgState = "iW" // iW

	// Install OK
	InstalledTriggerPending PkgState = "it" // it
	InstalledOK             PkgState = "ii" // ii

	// Hold depend failed
	HoldHalf     PkgState = "hH" // hH
	HoldUnpacked PkgState = "hU" // hU

	// Hold configure failed
	HoldConfigPending PkgState = "hF" // hF
	HoldTriggerAwait  PkgState = "hW" // hW

	// Hold and Install OK
	HoldTrigerPending PkgState = "ht" // ht
	HoldInstalled     PkgState = "hi" // hi
	HoldPurged        PkgState = "hc" // hc

	// Remove OK
	Removed PkgState = "rc" // rc

	// Remove depend failed
	RemoveHalf PkgState = "rH" // rH

	// Purged OK
	Purged     PkgState = "pc" // pc
	PurgedHalf PkgState = "pH" // pH

	// The intermediate state of the upgrade is not affected
	OnlyConfigFiles PkgState = "ic" // ic

)

func (s PkgState) CheckOK() bool {

	switch {
	case s == "":
		return false
	case s == InstalledOK, s == HoldInstalled, !(s == RemoveHalf || s == PurgedHalf) && len(s) == 2 && (s[:1] == "r" || s[:1] == "p"), s == HoldPurged, s == HoldTrigerPending, s == InstalledTriggerPending, s == OnlyConfigFiles:
		return true
	default:
		return false
	}
}

func (s PkgState) CheckFailed() (bool, error) {
	switch {
	case s == InstallHalf, s == InstallUnpacked, s == PurgedHalf, s == RemoveHalf, s == HoldHalf, s == HoldUnpacked:
		return true, nil
	case s.CheckOK():
		return false, nil
	case s == "":
		return false, fmt.Errorf("empty state")
	default:
		return false, fmt.Errorf("unknown state:%s", s)
	}
}

func (s PkgState) CheckConfigure() (bool, error) {
	switch {
	case s == InstallConfigPending, s == HoldConfigPending, s == HoldTriggerAwait, s == InstallTriggerAwait:
		return true, nil
	case s.CheckOK():
		return false, nil
	case s == "":
		return false, fmt.Errorf("empty state")
	default:
		return false, fmt.Errorf("unknown state:%s", s)
	}
}

type AppState struct {
	State PkgState `json:"state" yaml:"state"` // state
	AppInfo
}
