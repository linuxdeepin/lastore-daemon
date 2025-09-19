package check

import (
	"fmt"

	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
)

const (
	// Stage1 stage 1
	Stage1 = "stage1"
	// Stage2 stage 2
	Stage2 = "stage2"

	lightdmProgram = "/usr/sbin/lightdm"
)

var programCheckMap = map[string][]string{
	Stage1: {
		lightdmProgram,
	},
	Stage2: {
		lightdmProgram,
	},
}

func CheckImportantProgress(stage string) (int64, error) {
	if programCheckList, ok := programCheckMap[stage]; ok {
		for _, program := range programCheckList {
			programPid, err := runcmd.RunnerOutput(10, "pidof", program)
			if err != nil {
				return ecode.CHK_PROGRAM_ERROR, err
			}
			if len(programPid) == 0 {
				return ecode.CHK_IMPORTANT_PROGRESS_NOT_RUNNING, fmt.Errorf("%s not running", program)
			}
		}
	} else {
		return ecode.CHK_PROGRAM_ERROR, fmt.Errorf("%s is error postcheck stage parameter", stage)
	}
	return ecode.CHK_PROGRAM_SUCCESS, nil
}
