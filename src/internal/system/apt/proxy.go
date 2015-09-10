package apt

import (
	"fmt"
	"internal/system"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type APTProxy struct {
	installJobs  map[string]aptCommand
	downloadJobs map[string]aptCommand
	removeJobs   map[string]aptCommand
	indicator    system.Indicator
}

func NewAPTProxy() system.System {
	p := &APTProxy{
		installJobs:  make(map[string]aptCommand),
		downloadJobs: make(map[string]aptCommand),
		removeJobs:   make(map[string]aptCommand),
	}
	return p
}

func ParseProgressInfo(id, line string) (system.ProgressInfo, error) {
	fs := strings.SplitN(line, ":", 4)
	switch fs[0] {
	case "dlstatus", "pmstatus":
		v, err := strconv.ParseFloat(fs[2], 64)
		if err != nil {
			return system.ProgressInfo{JobId: id},
				fmt.Errorf("W: unknow progress value: %q", line)
		}
		return system.ProgressInfo{
			JobId:       id,
			Progress:    v / 100.0,
			Description: fs[3],
			Status:      system.RunningStatus,
		}, nil
	case "dstatus":
		switch fs[1] {
		case system.SuccessedStatus:
			return system.ProgressInfo{
				JobId:       id,
				Progress:    1.0,
				Description: fs[2],
				Status:      system.SuccessedStatus,
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
		fmt.Errorf("W: unknow status:", line)

}

func (p *APTProxy) AttachIndicator(f system.Indicator) {
	p.indicator = f
}

func (p *APTProxy) Download(jobId string, packageId string) error {
	p.downloadJobs[jobId] = buildDownloadCommand(
		func(info system.ProgressInfo) {
			info.JobId = jobId
			p.indicator(info)
			if info.Status == system.SuccessedStatus ||
				info.Status == system.FailedStatus {
				delete(p.downloadJobs, jobId)
			}
		}, packageId,
	)
	return nil
}

func (p *APTProxy) Remove(jobId string, packageId string) error {
	p.removeJobs[jobId] = buildRemoveCommand(
		func(info system.ProgressInfo) {
			info.JobId = jobId
			p.indicator(info)
			if info.Status == system.SuccessedStatus ||
				info.Status == system.FailedStatus {
				delete(p.removeJobs, jobId)
			}
		}, packageId,
	)
	return nil
}

func (p *APTProxy) Install(jobId string, packageId string) error {
	p.installJobs[jobId] = buildInstallCommand(
		func(info system.ProgressInfo) {
			info.JobId = jobId
			p.indicator(info)
			if info.Status == system.SuccessedStatus ||
				info.Status == system.FailedStatus {
				delete(p.installJobs, jobId)
			}
		}, packageId,
	)

	return nil
}

func (APTProxy) SystemUpgrade() {
}

func (p *APTProxy) Pause(jobId string) error {
	return system.NotImplementError
}

func (p *APTProxy) Start(jobId string) error {
	if c, ok := p.downloadJobs[jobId]; ok {
		c.Start()
		return nil
	}
	if c, ok := p.installJobs[jobId]; ok {
		c.Start()
		return nil
	}
	if c, ok := p.removeJobs[jobId]; ok {
		c.Start()
		return nil
	}
	return system.NotFoundError
}

func (p *APTProxy) Abort(jobId string) error {
	return system.NotImplementError
}

func (p *APTProxy) CheckInstalled(pid string) bool {
	out, err := exec.Command("dpkg-query", "-W", "-f", "${Status}", pid).Output()
	log.Println("CheckExits:", string(out))
	if err != nil {
		log.Println("CheckExists E:", pid, err)
		return false
	}
	if strings.Contains(string(out), "ok not-installed") {
		return false
	} else if strings.Contains(string(out), "install ok installed") {
		return true
	}
	return false
}

func (p *APTProxy) SystemArchitectures() []system.Architecture {
	bs, err := ioutil.ReadFile("/var/lib/dpkg/arch")
	if err != nil {
		log.Fatalln("Can't detect system architectures:", err)
		os.Exit(1)
	}
	var r []system.Architecture
	for _, arch := range strings.Split(string(bs), "\n") {
		i := strings.TrimSpace(arch)
		if i == "" {
			continue
		}
		r = append(r, system.Architecture(i))
	}
	return r
}
