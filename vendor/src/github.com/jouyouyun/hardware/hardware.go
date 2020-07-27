package hardware

import (
	"encoding/json"
	"fmt"

	hdisk "github.com/jouyouyun/hardware/disk"
	hdmi "github.com/jouyouyun/hardware/dmi"
	"github.com/jouyouyun/hardware/utils"
)

var (
	_mid            string
	IncludeDiskInfo bool
)

// GenMachineID generate this machine's id
func GenMachineID() (string, error) {
	if len(_mid) != 0 {
		return _mid, nil
	}

	dmi, err := hdmi.GetDMI()
	if !IncludeDiskInfo && err == nil && len(dmi.ProductUUID) != 0 {
		mid, err := genMachineIDWithDMI(*dmi)
		if err == nil {
			_mid = mid
			return mid, nil
		}
	} else if dmi == nil {
		dmi = &hdmi.DMI{}
	}

	// if dmi product uuid null or IncludeDiskInfo is true, generate machine id with root disk serial
	disks, err := hdisk.GetDiskList()
	if err != nil {
		return "", err
	}
	root := disks.GetRoot()
	if root == nil {
		return "", fmt.Errorf("no root disk found")
	}
	return genMachineIDWithDisk(*dmi, root)
}

func genMachineIDWithDMI(dmi hdmi.DMI) (string, error) {
	// bios info maybe changed after upgraded
	dmi.BiosDate = ""
	dmi.BiosVendor = ""
	dmi.BiosVersion = ""
	return doGenMachineID(&dmi)
}

func genMachineIDWithDisk(dmi hdmi.DMI, disk *hdisk.Disk) (string, error) {
	// bios info maybe changed after upgraded
	dmi.BiosDate = ""
	dmi.BiosVendor = ""
	dmi.BiosVersion = ""
	var info = struct {
		hdmi.DMI
		DiskSerial string
	}{
		DMI:        dmi,
		DiskSerial: disk.Serial,
	}
	return doGenMachineID(&info)
}

func doGenMachineID(info interface{}) (string, error) {
	data, err := json.Marshal(info)
	if err != nil {
		return "", err
	}
	return utils.SHA256Sum(data), nil
}
