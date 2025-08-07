package fixpkg

import (
	"fmt"

	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
	// "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
)

func FixConfig(outfile string) error {
	// dpkg --configure -a

	Argv := fmt.Sprintf("/usr/bin/dpkg --force-confold --skip-same-version --configure -a >%s 2>&1", outfile)
	_, err := runcmd.RunnerOutputEnv(3600, "/usr/bin/bash", []string{"DEBIAN_FRONTEND=noninteractive", "DEBCONF_NONINTERACTIVE_SEEN=true", "DEBIAN_PRIORITY=critical"}, "-c", Argv)
	if err != nil {
		return fmt.Errorf("fix/cfg package configure error: %v", err)
	}

	return nil
}
