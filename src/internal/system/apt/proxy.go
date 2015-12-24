package apt

import (
	"fmt"
	log "github.com/cihub/seelog"
	"internal/system"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

type APTSystem struct {
	cmdSet    map[string]*aptCommand
	indicator system.Indicator
}

func New() system.System {
	p := &APTSystem{
		cmdSet: make(map[string]*aptCommand),
	}
	return p
}

func parseProgressField(v string) (float64, error) {
	progress, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return -1, fmt.Errorf("unknown progress value: %q", v)
	}
	return progress, nil

}
func ParseProgressInfo(id, line string) (system.JobProgressInfo, error) {
	fs := strings.SplitN(line, ":", 4)
	if len(fs) != 4 {
		return system.JobProgressInfo{JobId: id}, fmt.Errorf("Invlaid Progress line:%q", line)
	}

	progress, err := parseProgressField(fs[2])
	if err != nil {
		return system.JobProgressInfo{JobId: id}, err
	}
	description := strings.TrimSpace(fs[3])

	var status system.Status
	var cancelable = true

	infoType := fs[0]

	switch infoType {
	case "dummy":
		status = system.Status(fs[1])
	case "dlstatus":
		progress = progress / 100.0 * 0.5
		status = system.RunningStatus
	case "pmstatus":
		progress = 0.5 + progress/100.0*0.5
		status = system.RunningStatus
		cancelable = false
	case "pmerror":
		progress = -1
		status = system.FailedStatus

	default:
		//	case "pmconffile", "media-change":
		return system.JobProgressInfo{JobId: id},
			fmt.Errorf("W: unknow status:%q", line)

	}

	return system.JobProgressInfo{
		JobId:       id,
		Progress:    progress,
		Description: description,
		Status:      status,
		Cancelable:  cancelable,
	}, nil
}

func (p *APTSystem) AttachIndicator(f system.Indicator) {
	p.indicator = f
}

func (p *APTSystem) Download(jobId string, packages []string) error {
	c := newAPTCommand(p, jobId, system.DownloadJobType, p.indicator, packages)
	return c.Start()
}

func (p *APTSystem) Remove(jobId string, packages []string) error {
	c := newAPTCommand(p, jobId, system.RemoveJobType, p.indicator, packages)
	return c.Start()
}

func (p *APTSystem) Install(jobId string, packages []string) error {
	c := newAPTCommand(p, jobId, system.InstallJobType, p.indicator, packages)
	return c.Start()
}

func (p *APTSystem) UpdateSource(jobId string) error {
	c := newAPTCommand(p, jobId, system.UpdateSourceJobType, p.indicator, nil)
	return c.Start()
}

func (p *APTSystem) Abort(jobId string) error {
	if c := p.FindCMD(jobId); c != nil {
		return c.Abort()
	}
	return system.NotFoundError
}

func (p *APTSystem) CheckInstallable(pkgId string) bool {
	out, err := exec.Command("/usr/bin/apt-cache", "show", pkgId).CombinedOutput()
	if err != nil {
		log.Debugf("CheckInstabllable(%q) failed: %q %v\n", pkgId, string(out), err)
		return false
	}
	return true
}
func (p *APTSystem) CheckInstalled(pkgId string) bool {
	out, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f", "${Status}", pkgId).CombinedOutput()
	if err != nil {
		return false
	}
	if strings.Contains(string(out), "ok not-installed") {
		return false
	} else if strings.Contains(string(out), "install ok installed") {
		return true
	}
	return false
}

func (p *APTSystem) SystemArchitectures() []system.Architecture {
	return getSystemArchitectures()
}

func (p *APTSystem) UpgradeInfo() []system.UpgradeInfo {
	var r []system.UpgradeInfo
	err := system.DecodeJson(path.Join(system.VarLibDir, "update_infos.json"),
		&r)
	if err != nil {
		log.Warnf("Invalid update_infos: %v\n", err)
	}
	return r
}
