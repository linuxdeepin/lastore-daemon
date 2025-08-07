package cache

import (
	"reflect"
)

type PState string

const (
	P_Init          PState = "Init"
	P_Stage0_Failed PState = "S0Failed"
	P_Stage1_Failed PState = "S1Failed"
	P_Stage2_Failed PState = "S2Failed"
	P_Stage3_Failed PState = "S3Failed"
	P_Stage4_Failed PState = "S4Failed"
	P_OK            PState = "Ok"
	P_Run           PState = "Run"
	P_Error         PState = "Error"
	P_Unknown       PState = "Unknown"
)

func (p PState) IsOk() bool {
	return p == P_OK
}

func (p PState) IsFault() bool {
	return !(p == P_Init || p == P_OK)
}

func (p PState) IsRunning() bool {
	return p == P_Run
}

func (p PState) IsFirstRun() bool {
	return (p == P_Init || p == "")
}

// InternalState 数据结构体
type InternalState struct {
	IsMetaInfoFormatCheck PState `json:"MetaInfoFormatCheck" yaml:"MetaInfoFormatCheck" default:"Init"` // metainfo-format-check
	IsDependsPreCheck     PState `json:"PkgDependsPreCheck" yaml:"PkgDependsPreCheck" default:"Init"`   // pkg-depends-precheck
	IsDependsMidCheck     PState `json:"PkgDependsMidCheck" yaml:"PkgDependsMidCheck" default:"Init"`   // pkg-depends-midcheck
	IsCVEOffline          PState `json:"CVEOffline" yaml:"CVEOffline" default:"init"`                   // CVEOffline
	// IsPkgFixed       bool   `json:"PkgFixed" yaml:"PkgFixed" default:"false"`                      // pkg-fixed
	// IsUpdateFetch    bool   `json:"UpdateFetch" yaml:"UpdateFetch" default:"false"`            // update-fetch
	// IsRepoFetch      bool   `json:"RepoFetched" yaml:"RepoFetched" default:"false"`                // repo-fetch
	IsEmulationCheck  bool   `json:"EmuCheck" yaml:"EmuCheck" default:"false"`                 // emu-checked
	IsDpkgAptPreCheck PState `json:"DpkgAptPreCheck" yaml:"DpkgAptPreCheck" default:"Init"`    // dpkg-apt-precheck
	IsDpkgAptMidCheck PState `json:"DpkgAptMidCheck" yaml:"DpkgAptMidCheck" default:"Init"`    // dpkg-apt-midcheck
	IsPreCheck        PState `cktag:"PreCheck" json:"PreCheck" yaml:"PreCheck" default:"Init"` // pre-check
	IsMidCheck        PState `json:"MidCheck" yaml:"MidCheck" default:"Init"`                  // mid-check
	IsPostCheckStage1 PState `json:"PostCheckStage1" yaml:"PostCheckStage1" default:"Init"`    // post-check
	IsPostCheckStage2 PState `json:"PostCheckStage2" yaml:"PostCheckStage2" default:"Init"`    // post-check
	IsInstallState    PState `json:"InstState" yaml:"InstState" default:"Init"`                // inst-state
	IsPurgeState      PState `json:"PurgeState" yaml:"PurgeState" default:"Init"`              // purge-state
}

func GetCheckTag(f reflect.StructField) string {
	return f.Tag.Get("cktag")
}
