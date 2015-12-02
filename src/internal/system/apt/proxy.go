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

func ParseProgressInfo(id, line string) (system.JobProgressInfo, error) {
	fs := strings.SplitN(line, ":", 4)
	switch fs[0] {
	case "dlstatus", "pmstatus", "dist_upgrade":
		v, err := strconv.ParseFloat(fs[2], 64)
		if err != nil {
			return system.JobProgressInfo{JobId: id},
				fmt.Errorf("W: unknow progress value: %q", line)
		}
		if v == -1 {
			return system.JobProgressInfo{JobId: id},
				fmt.Errorf("W: failed: %q", line)
		}
		return system.JobProgressInfo{
			JobId:       id,
			Progress:    v / 100.0,
			Description: strings.TrimSpace(fs[3]),
			Status:      system.RunningStatus,
			Cancelable:  fs[0] == "dlstatus",
		}, nil
	case "dstatus":
		switch fs[1] {
		case system.SucceedStatus:
			return system.JobProgressInfo{
				JobId:       id,
				Progress:    1.0,
				Description: strings.TrimSpace(fs[2]),
				Status:      system.SucceedStatus,
				Cancelable:  fs[0] == "dlstatus",
			}, nil
		case system.FailedStatus, system.PausedStatus:
			return system.JobProgressInfo{
				JobId:       id,
				Progress:    -1,
				Description: strings.TrimSpace(fs[2]),
				Status:      system.Status(fs[1]),
				Cancelable:  true,
			}, nil
		}
	}
	return system.JobProgressInfo{JobId: id},
		fmt.Errorf("W: unknow status:%q", line)
}

func (p *APTSystem) AttachIndicator(f system.Indicator) {
	p.indicator = f
}

func (p *APTSystem) Download(jobId string, packageId string) error {
	c := newAPTCommand(p, jobId, system.DownloadJobType, p.indicator, packageId)
	return c.Start()
}

func (p *APTSystem) Remove(jobId string, packageId string) error {
	c := newAPTCommand(p, jobId, system.RemoveJobType, p.indicator, packageId)
	return c.Start()
}

func (p *APTSystem) Install(jobId string, packageId string) error {
	c := newAPTCommand(p, jobId, system.InstallJobType, p.indicator, packageId)
	return c.Start()
}

func (p *APTSystem) DistUpgrade(jobId string) error {
	c := newAPTCommand(p, jobId, system.DistUpgradeJobType, p.indicator, "")
	return c.Start()
}

func (p *APTSystem) UpdateSource(jobId string) error {
	c := newAPTCommand(p, jobId, system.UpdateSourceJobType, p.indicator, "")
	return c.Start()
}

func (p *APTSystem) Abort(jobId string) error {
	if c := p.FindCMD(jobId); c != nil {
		return c.Abort()
	}
	return system.NotFoundError
}

func (p *APTSystem) CheckInstalled(pid string) bool {
	out, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f", "${Status}", pid).CombinedOutput()
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