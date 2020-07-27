package bluetooth

import (
	"github.com/jouyouyun/hardware/utils"
)

const (
	blueSysfsDir = "/sys/class/bluetooth"
)

// Bluetooth store bluetooth info
type Bluetooth struct {
	utils.CardInfo
}

// BluetoothList store bluetooth list
type BluetoothList []*Bluetooth

// GetBluetoothList return bluetooth device list
func GetBluetoothList() (BluetoothList, error) {
	list, err := utils.ScanDir(blueSysfsDir, utils.FilterBluetoothName)
	if err != nil {
		return nil, err
	}
	var blueList BluetoothList
	for _, name := range list {
		blue, err := newBluetooth(blueSysfsDir, name)
		if err != nil {
			return nil, err
		}
		blueList = append(blueList, blue)
	}
	return blueList, nil
}

func newBluetooth(dir, name string) (*Bluetooth, error) {
	card, err := utils.NewCardInfo(dir, name)
	if err != nil {
		return nil, err
	}
	return &Bluetooth{CardInfo: *card}, nil
}
