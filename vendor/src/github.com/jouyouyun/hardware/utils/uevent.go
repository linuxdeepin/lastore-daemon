package utils

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
)

// UEventType device port r=type
type UEventType int

const (
	// device with pci port
	UEventTypePCI UEventType = iota + 11
	// device with usb port
	UEventTypeUSB
)

const (
	// SlotTypePCI pci type slot
	SlotTypePCI = "pci"
	// SlotTypeUSB usb type slot
	SlotTypeUSB = "usb"
)

// IDInfo store device vendor or device info
type IDInfo struct {
	ID   string
	Name string
}

// PCIUevent pci uevent data
type PCIUEvent struct {
	Driver  string
	Vendor  *IDInfo
	Device  *IDInfo
	SVendor *IDInfo // subsystem vendor
	SDevice *IDInfo // subsystem device

	name     string
	slotName string
}

// USBUEvent usb uevent data
type USBUEvent struct {
	Driver  string
	Vendor  string
	Product string

	name string
}

// UEvent store device uevent file
type UEvent struct {
	Type UEventType
	Name string
	Data interface{}
}

func NewUEvent(filename string) (*UEvent, error) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var pairs = make(map[string]string)
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		items := strings.SplitN(line, "=", 2)
		pairs[items[0]] = items[1]
	}

	var info UEvent
	if _, ok := pairs["PCI_SLOT_NAME"]; ok {
		info.Type = UEventTypePCI
		var pci *PCIUEvent
		pci, err = newPCIUEvent(pairs)
		if err == nil {
			info.Name = pci.name
			info.Data = pci
		}
	} else {
		var usb *USBUEvent
		info.Type = UEventTypeUSB
		usb, err = newUSBUEvent(pairs)
		if err == nil {
			info.Name = usb.name
			info.Data = usb
		}
	}
	if err != nil {
		return nil, err
	}

	return &info, nil
}

func newPCIUEvent(pairs map[string]string) (*PCIUEvent, error) {
	var info = PCIUEvent{
		Driver:   pairs["DRIVER"],
		slotName: pairs["PCI_SLOT_NAME"],
	}
	output, err := getCommandOutput(fmt.Sprintf("lspci -vmm -s %s", info.slotName))
	if err != nil {
		return nil, err
	}
	outPairs := formatLspciOutput(output)

	pciID := pairs["PCI_ID"]
	idItems := strings.Split(pciID, ":")
	info.Vendor = &IDInfo{ID: idItems[0], Name: outPairs["Vendor"]}
	info.Device = &IDInfo{ID: idItems[1], Name: outPairs["Device"]}

	subsysID := pairs["PCI_SUBSYS_ID"]
	subsysItems := strings.Split(subsysID, ":")
	info.SVendor = &IDInfo{ID: subsysItems[0], Name: outPairs["SVendor"]}
	info.SDevice = &IDInfo{ID: subsysItems[1], Name: outPairs["SDevice"]}

	info.name = fmt.Sprintf("%s %s", info.Vendor.Name, info.Device.Name)
	return &info, nil
}

func newUSBUEvent(pairs map[string]string) (*USBUEvent, error) {
	var info = USBUEvent{
		Driver: pairs["DRIVER"],
	}
	product := pairs["PRODUCT"]
	items := strings.Split(product, "/")
	if len(items) < 3 {
		return nil, fmt.Errorf("invalid uevent format, items < 3")
	}

	// compatible usb mouse
	idx := 0
	if len(items) == 4 {
		idx = 1
	}
	info.Vendor = fmt.Sprintf("%04s", items[idx])
	info.Product = fmt.Sprintf("%04s", items[idx+1])

	output, err := getCommandOutput(fmt.Sprintf("lsusb -d %s:%s",
		info.Vendor, info.Product))
	if err != nil {
		return nil, err
	}
	info.name = formatLsusbOutput(output)

	return &info, nil
}

func formatLspciOutput(output []byte) map[string]string {
	lines := strings.Split(string(output), "\n")
	var pairs = make(map[string]string)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		items := strings.SplitN(line, ":", 2)
		items[1] = strings.TrimSpace(items[1])
		pairs[items[0]] = items[1]
	}
	return pairs
}

func formatLsusbOutput(output []byte) string {
	line := string(output)
	line = strings.TrimRight(line, "\n")
	items := strings.Split(line, "ID ")
	list := strings.SplitN(items[1], " ", 2)
	if len(list) != 2 {
		return ""
	}
	return list[1]
}

func getCommandOutput(cmd string) ([]byte, error) {
	return exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
}
