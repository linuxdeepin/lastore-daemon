package camera

import (
	"github.com/jouyouyun/hardware/utils"
)

const (
	cameraSysfsDir = "/sys/class/video4linux"
)

// Camera store camera device info
type Camera struct {
	utils.CardInfo
}

// CameraList store camera device list
type CameraList []*Camera

// GetCameraList return camera device list
func GetCameraList() (CameraList, error) {
	list, err := utils.ScanDir(cameraSysfsDir, utils.FilterCameraName)
	if err != nil {
		return nil, err
	}
	var cards CameraList
	for _, name := range list {
		card, err := newCamera(cameraSysfsDir, name)
		if err != nil {
			return nil, err
		}
		cards = cards.Append(card)
	}
	return cards, nil
}

func newCamera(dir, name string) (*Camera, error) {
	card, err := utils.NewCardInfo(cameraSysfsDir, name)
	if err != nil {
		return nil, err
	}
	return &Camera{CardInfo: *card}, nil
}

// Append append camera device after filter duplication device
// In X230 has two same camera device
func (list CameraList) Append(card *Camera) CameraList {
	for _, v := range list {
		if v.Equal(card) {
			return list
		}
	}
	list = append(list, card)
	return list
}

// Equal check device whether equal by vendor and product
func (c *Camera) Equal(tmp *Camera) bool {
	return c.Vendor == tmp.Vendor && c.Product == tmp.Product
}
