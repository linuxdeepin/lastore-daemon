package apt

import (
	"fmt"
	"internal/system"
	"os/exec"
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

func ParseProgressInfo(id, line string) (system.ProgressInfo, error) {
	fs := strings.SplitN(line, ":", 4)
	switch fs[0] {
	case "dlstatus", "pmstatus", "dist_upgrade":
		v, err := strconv.ParseFloat(fs[2], 64)
		if err != nil {
			return system.ProgressInfo{JobId: id},
				fmt.Errorf("W: unknow progress value: %q", line)
		}
		if v == -1 {
			return system.ProgressInfo{JobId: id},
				fmt.Errorf("W: failed: %q", line)
		}
		return system.ProgressInfo{
			JobId:       id,
			Progress:    v / 100.0,
			Description: fs[3],
			Status:      system.RunningStatus,
		}, nil
	case "dstatus":
		switch fs[1] {
		case system.SucceedStatus:
			return system.ProgressInfo{
				JobId:       id,
				Progress:    1.0,
				Description: fs[2],
				Status:      system.SucceedStatus,
			}, nil
		case system.FailedStatus:
			return system.ProgressInfo{
				JobId:       id,
				Progress:    -1,
				Description: fs[2],
				Status:      system.FailedStatus,
			}, nil
		}
	}
	return system.ProgressInfo{JobId: id},
		fmt.Errorf("W: unknow status:%q", line)

}

func (p *APTSystem) AttachIndicator(f system.Indicator) {
	p.indicator = f
}

func (p *APTSystem) Download(jobId string, packageId string, region string) error {
	newAPTCommand(p, jobId, system.DownloadJobType, p.indicator, packageId, region)
	return nil
}

func (p *APTSystem) Remove(jobId string, packageId string) error {
	newAPTCommand(p, jobId, system.RemoveJobType, p.indicator, packageId, "")
	return nil
}

func (p *APTSystem) Install(jobId string, packageId string) error {
	newAPTCommand(p, jobId, system.InstallJobType, p.indicator, packageId, "")
	return nil
}

func (p *APTSystem) DistUpgrade() error {
	const DistUpgradeJobId = "dist_upgrade"
	// TODO: clean
	c := newAPTCommand(p, DistUpgradeJobId, system.DistUpgradeJobType, p.indicator, "", "")
	c.Start()
	return nil
}

func (p *APTSystem) Pause(jobId string) error {
	return system.NotImplementError
}

func (p *APTSystem) Start(jobId string) error {
	if c := p.FindCMD(jobId); c != nil {
		c.Start()
		return nil
	}
	return system.NotFoundError
}

func (p *APTSystem) Abort(jobId string) error {
	return system.NotImplementError
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
	return mapUpgradeInfo(
		queryDpkgUpgradeInfoByAptList(),
		buildUpgradeInfoRegex(getSystemArchitectures()),
		buildUpgradeInfo)
}
