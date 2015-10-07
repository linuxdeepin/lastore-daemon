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
		fmt.Errorf("W: unknow status:%q", line)

}

func (p *APTSystem) AttachIndicator(f system.Indicator) {
	p.indicator = f
}

func (p *APTSystem) Download(jobId string, packageId string, region string) error {
	newAPTCommand(p, jobId, "download", p.indicator, packageId, region)
	return nil
}

func (p *APTSystem) Remove(jobId string, packageId string) error {
	newAPTCommand(p, jobId, "remove", p.indicator, packageId, "")
	return nil
}

func (p *APTSystem) Install(jobId string, packageId string) error {
	newAPTCommand(p, jobId, "install", p.indicator, packageId, "")
	return nil
}

func (APTSystem) SystemUpgrade() {
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
