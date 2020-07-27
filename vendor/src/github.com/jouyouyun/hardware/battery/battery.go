package battery

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jouyouyun/hardware/utils"
)

const (
	batterySysfsDir = "/sys/class/power_supply"
)

// Battery store battery info
type Battery struct {
	Name         string
	Model        string
	Manufacturer string
	Serial       string
	Technology   string

	CapacityDesign int64
	CapacityNow    int64
}

// BatteryList store battery list
type BatteryList []*Battery

// GetBatteryList return battery list
func GetBatteryList() (BatteryList, error) {
	list, err := utils.ScanDir(batterySysfsDir, func(string) bool {
		return false
	})
	if err != nil {
		return nil, err
	}
	var batList BatteryList
	for _, name := range list {
		if !isBattery(batterySysfsDir, name) {
			continue
		}
		bat, err := newBattery(batterySysfsDir, name)
		if err != nil {
			return nil, err
		}
		batList = append(batList, bat)
	}
	return batList, nil
}

func newBattery(dir, name string) (*Battery, error) {
	uevent := filepath.Join(dir, name, "uevent")
	set, err := parseFile(uevent)
	if err != nil {
		return nil, err
	}

	var bat = Battery{
		Name:         set["POWER_SUPPLY_NAME"],
		Model:        set["POWER_SUPPLY_MODEL_NAME"],
		Manufacturer: set["POWER_SUPPLY_MANUFACTURER"],
		Serial:       set["POWER_SUPPLY_SERIAL_NUMBER"],
		Technology:   set["POWER_SUPPLY_TECHNOLOGY"],
	}
	bat.CapacityDesign, _ = strconv.ParseInt(set["POWER_SUPPLY_CHARGE_FULL_DESIGN"],
		10, 64)
	bat.CapacityNow, _ = strconv.ParseInt(set["POWER_SUPPLY_CHARGE_NOW"],
		10, 64)
	return &bat, nil
}

func isBattery(dir, name string) bool {
	filename := filepath.Join(dir, name, "type")
	ty, err := utils.ReadFileContent(filename)
	if err != nil {
		return false
	}
	return ty == "Battery"
}

func parseFile(filename string) (map[string]string, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var set = make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		items := strings.Split(line, "=")
		if len(items) != 2 {
			continue
		}
		set[items[0]] = items[1]
	}
	return set, nil
}
