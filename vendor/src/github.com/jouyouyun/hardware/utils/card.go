package utils

import (
	"path/filepath"
	"strconv"
)

// CardInfo store card info, such as: sound, graphic...
type CardInfo struct {
	Name    string
	Vendor  string
	Product string
	Slot    string
}

// NewCardInfo create card info from uevent
func NewCardInfo(dir, name string) (*CardInfo, error) {
	uevent := filepath.Join(dir, name, "device", "uevent")
	uinfo, err := NewUEvent(uevent)
	if err != nil {
		return nil, err
	}

	var card = CardInfo{Name: uinfo.Name}
	switch uinfo.Type {
	case UEventTypePCI:
		pci := uinfo.Data.(*PCIUEvent)
		card.Vendor = pci.Vendor.ID
		card.Product = pci.Device.ID
		card.Slot = SlotTypePCI
	case UEventTypeUSB:
		usb := uinfo.Data.(*USBUEvent)
		card.Vendor = usb.Vendor
		card.Product = usb.Product
		card.Slot = SlotTypeUSB
	}

	return &card, nil
}

// FilterCardName filter sound,graphic card name, such as 'card0'
func FilterCardName(name string) bool {
	return filterName(name, "card")
}

// FilterBluetoothName filter bluetooth device name
func FilterBluetoothName(name string) bool {
	return filterName(name, "hci")
}

// FilterCameraName filter camera device name
func FilterCameraName(name string) bool {
	return filterName(name, "video")
}

func filterName(name, key string) bool {
	klen := len(key)
	if len(name) <= klen {
		return true
	}

	tmp := string([]byte(name)[:klen])
	if tmp != key {
		return true
	}
	// number
	tmp = string([]byte(name)[klen:])
	_, err := strconv.Atoi(tmp)
	if err != nil {
		return true
	}
	return false
}
