package dut

import (
	"bytes"
	"encoding/json"
	"internal/system"
	"internal/system/apt"
	"io/ioutil"
	"os/exec"
	"sync"
	"time"

	"github.com/linuxdeepin/go-lib/utils"
)

type DutSystem struct {
	apt.APTSystem
}

func NewSystem(systemSourceList []string, nonUnknownList []string, otherList []string) system.System {
	aptImpl := apt.New(systemSourceList, nonUnknownList, otherList)
	return &DutSystem{
		APTSystem: aptImpl,
	}
}

func OptionToArgs(options map[string]string) []string {
	var args []string
	for key, value := range options { // dut 命令执行参数
		args = append(args, key)
		args = append(args, value)
	}
	return args
}

func (p *DutSystem) UpdateSource(jobId string, environ map[string]string, args map[string]string) error {
	err := checkSystemDependsError()
	if err != nil {
		return err
	}
	return p.APTSystem.UpdateSource(jobId, environ, args)
}

func (p *DutSystem) DistUpgrade(jobId string, environ map[string]string, args map[string]string) error {
	err := checkSystemDependsError()
	if err != nil {
		return err
	}
	c := newDUTCommand(p, jobId, system.DistUpgradeJobType, p.Indicator, OptionToArgs(args))
	c.SetEnv(environ)
	return c.Start()
}

func (p *DutSystem) FixError(jobId string, errType string, environ map[string]string, args map[string]string) error {
	c := newDUTCommand(p, jobId, system.FixErrorJobType, p.Indicator, append([]string{errType}, OptionToArgs(args)...))
	c.SetEnv(environ)
	return c.Start()
}

func (p *DutSystem) CheckSystem(jobId string, checkType string, environ map[string]string, args map[string]string) error {
	c := newDUTCommand(p, jobId, system.CheckSystemJobType, p.Indicator, OptionToArgs(args))
	c.SetEnv(environ)
	return c.Start()
}

func checkSystemDependsError() error {
	cmd := exec.Command("deepin-system-fixpkg", "check")
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err == nil {
		err = apt.CheckPkgSystemError(false)
		if err != nil {
			return &system.JobError{
				Type:   system.ErrorUnknown,
				Detail: err.Error(),
			}
		}
		return nil
	}
	return parsePkgSystemError(outBuf.String(), errBuf.String())
}

type checkType uint

const (
	PreCheck  checkType = 0
	MidCheck  checkType = 1
	PostCheck checkType = 2
)

func (t checkType) String() string {
	switch t {
	case PreCheck:
		return "precheck"
	case MidCheck:
		return "midcheck"
	case PostCheck:
		return "postcheck"
	}
	return ""
}

type RuleInfo struct {
	Name    string
	Type    checkType
	Command string
	Argv    string
}

type RepoInfo struct {
	Name       string
	FilePath   string
	HashSha256 string
}

type metaInfo struct {
	PkgDebPath string
	PkgList    []system.PackageInfo
	CoreList   []system.PackageInfo
	OptionList []system.PackageInfo
	BaseLine   []system.PackageInfo
	Rules      []RuleInfo
	ReposInfo  []RepoInfo `json:"RepoInfo"`
	UUID       string
	Time       string
}

const (
	strict      = "strict"
	skipState   = "skipstate"
	skipVersion = "skipversion"
	exist       = "exist"
)

// GenDutMetaFile metaPath为DutOnlineMetaConfPath或DutOfflineMetaConfPath
func GenDutMetaFile(metaPath, debPath string, pkgMap, coreMap, optionMap, baseMap map[string]system.PackageInfo, rules []RuleInfo, repoInfo []RepoInfo) (string, error) {
	meta := metaInfo{
		PkgDebPath: debPath,
		UUID:       utils.GenUuid(),
		Time:       time.Now().String(),
		Rules:      rules,
		ReposInfo:  repoInfo,
	}
	var pkgList []system.PackageInfo
	var coreList []system.PackageInfo
	var optionList []system.PackageInfo
	var baseList []system.PackageInfo
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		pkgList = genPkgList(pkgMap)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		coreList = genCoreList(coreMap)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		optionList = genOptionList(optionMap)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		baseList = genBaseList(baseMap)
		wg.Done()
	}()
	wg.Wait()
	meta.PkgList = pkgList
	meta.CoreList = coreList
	meta.OptionList = optionList
	meta.BaseLine = baseList
	content, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	err = ioutil.WriteFile(metaPath, content, 0644)
	if err != nil {
		return "", err
	}
	return meta.UUID, nil
}

func genPkgList(pkgMap map[string]system.PackageInfo) []system.PackageInfo {
	var list []system.PackageInfo
	for _, v := range pkgMap {
		info := system.PackageInfo{
			Name:    v.Name,
			Version: v.Version,
			Need:    v.Need,
		}
		list = append(list, info)
	}
	return list
}
func genCoreList(pkgMap map[string]system.PackageInfo) []system.PackageInfo {
	var list []system.PackageInfo
	for _, v := range pkgMap {
		info := system.PackageInfo{
			Name:    v.Name,
			Version: v.Version,
			Need:    skipVersion,
		}
		list = append(list, info)
	}
	return list
}
func genOptionList(pkgMap map[string]system.PackageInfo) []system.PackageInfo {
	var list []system.PackageInfo
	for _, v := range pkgMap {
		info := system.PackageInfo{
			Name:    v.Name,
			Version: v.Version,
			Need:    exist,
		}
		list = append(list, info)
	}
	return list
}
func genBaseList(pkgMap map[string]system.PackageInfo) []system.PackageInfo {
	var list []system.PackageInfo
	for _, v := range pkgMap {
		info := system.PackageInfo{
			Name:    v.Name,
			Version: v.Version,
			Need:    skipVersion,
		}
		list = append(list, info)
	}
	return list
}

func GetDutErrorMessage() string {
	return "TODO dut error message"
}
