package check

import (
	"fmt"

	"github.com/godbus/dbus/v5"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
	"github.com/linuxdeepin/go-lib/dbusutil"
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

var serviceCheckMap = map[string][]string{
	Stage1: {
		"display-manager.service",
	},
	Stage2: {
		"display-manager.service",
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

type SystemdChecker struct {
	conn    *dbus.Conn
	manager systemd1.Manager
}

func NewSystemdChecker() (*SystemdChecker, error) {
	service, err := dbusutil.NewSystemService()
	if err != nil {
		return nil, err
	}

	conn := service.Conn()

	return &SystemdChecker{
		conn:    conn,
		manager: systemd1.NewManager(conn),
	}, nil
}

func (c *SystemdChecker) IsUnitActive(serviceName string) (bool, error) {
	unitPath, err := c.manager.GetUnit(0, serviceName)
	if err != nil {
		return false, nil
	}

	unit, err := systemd1.NewUnit(c.conn, unitPath)
	if err != nil {
		return false, err
	}

	activeState, err := unit.Unit().ActiveState().Get(0)
	if err != nil {
		return false, err
	}

	return activeState == "active", nil
}

func CheckImportantService(stage string) error {
	SystemdChecker, err := NewSystemdChecker()
	if err != nil {
		return &system.JobError{
			ErrType:      system.ErrorCheckServiceFailed,
			ErrDetail:    fmt.Sprintf("create systemd checker failed: %v", err),
			IsCheckError: true,
		}
	}

	if serviceCheckList, ok := serviceCheckMap[stage]; ok {
		for _, service := range serviceCheckList {
			active, err := SystemdChecker.IsUnitActive(service)
			if err != nil {
				return &system.JobError{
					ErrType:      system.ErrorCheckServiceFailed,
					ErrDetail:    fmt.Sprintf("check important service error: %v", err),
					IsCheckError: true,
				}
			}
			if !active {
				return &system.JobError{
					ErrType:      system.ErrorCheckServiceFailed,
					ErrDetail:    fmt.Sprintf("%s not running", service),
					IsCheckError: true,
				}
			}
		}
	} else {
		return &system.JobError{
			ErrType:      system.ErrorCheckServiceFailed,
			ErrDetail:    fmt.Sprintf("%s is error postcheck stage parameter", stage),
			IsCheckError: true,
		}
	}
	return nil
}
