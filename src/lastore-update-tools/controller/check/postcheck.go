package check

import (
	"fmt"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
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

func CheckImportantProcess(stage string) error {
	if programCheckList, ok := programCheckMap[stage]; ok {
		for _, program := range programCheckList {
			programPid, err := runcmd.RunnerOutput(10, "pidof", program)
			if err != nil {
				return &system.JobError{
					ErrType:      system.ErrorCheckProgramFailed,
					ErrDetail:    fmt.Sprintf("check important progress error: %v", err),
					IsCheckError: true,
				}
			}
			if len(programPid) == 0 {
				return &system.JobError{
					ErrType:      system.ErrorCheckProcessNotRunning,
					ErrDetail:    fmt.Sprintf("%s not running", program),
					IsCheckError: true,
				}
			}
		}
	} else {
		return &system.JobError{
			ErrType:      system.ErrorCheckProgramFailed,
			ErrDetail:    fmt.Sprintf("%s is error postcheck stage parameter", stage),
			IsCheckError: true,
		}
	}
	return nil
}
