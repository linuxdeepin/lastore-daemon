package peripherals

import (
	"fmt"
	"io/ioutil"
	"strings"
)

// Peripherals store input/output device
type Peripherals struct {
	Name    string
	Vendor  string
	Product string
}

// PeripheralsList store input/output device list
type PeripheralsList []*Peripherals

const (
	peripheralsFilename = "/proc/bus/input/devices"
)

// GetPeripheralsList return peripherals device list
func GetPeripheralsList() (PeripheralsList, error) {
	segmentList, err := getSegmentList(peripheralsFilename)
	if err != nil {
		return nil, err
	}
	var infos PeripheralsList
	for _, segment := range segmentList {
		if len(segment) == 0 {
			continue
		}
		info, err := newPerpherals(segment)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func newPerpherals(segment string) (*Peripherals, error) {
	set := parseSegment(segment)
	if len(set) == 0 {
		return nil, fmt.Errorf("invalid segment: %s", segment)
	}
	var info = Peripherals{
		Name:    set["Name"],
		Vendor:  set["Vendor"],
		Product: set["Product"],
	}
	info.Name = strings.TrimLeft(info.Name, "\"")
	info.Name = strings.TrimRight(info.Name, "\"")
	return &info, nil
}

func parseSegment(segment string) map[string]string {
	var set = make(map[string]string)
	lines := strings.Split(segment, "\n")
	parseFirstLine(set, lines[0])
	for i := 1; i < len(lines); i++ {
		if len(lines[i]) == 0 {
			continue
		}
		items1 := strings.SplitN(lines[i], ": ", 2)
		if len(items1) != 2 {
			continue
		}
		items2 := strings.SplitN(items1[1], "=", 2)
		if len(items2) != 2 {
			continue
		}
		set[items2[0]] = items2[1]
	}
	return set
}

func parseFirstLine(set map[string]string, line string) {
	items := strings.SplitN(line, ": ", 2)
	if len(items) != 2 {
		return
	}
	items2 := strings.Split(items[1], " ")
	if len(items2) != 4 {
		return
	}
	for i := 1; i < 3; i++ {
		list := strings.Split(items2[i], "=")
		set[list[0]] = list[1]
	}
}

func getSegmentList(filename string) ([]string, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(content), "\n\n"), nil
}
