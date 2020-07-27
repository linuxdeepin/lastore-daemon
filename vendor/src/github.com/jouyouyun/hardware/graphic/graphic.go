package graphic

import (
	"github.com/jouyouyun/hardware/utils"
)

const (
	graphicSysfsDir = "/sys/class/drm"
)

// Graphic store graphic card info
type Graphic struct {
	utils.CardInfo
}

// GraphicList store graphic card list
type GraphicList []*Graphic

// GetGraphicList return card list
func GetGraphicList() (GraphicList, error) {
	list, err := utils.ScanDir(graphicSysfsDir, utils.FilterCardName)
	if err != nil {
		return nil, err
	}

	var cards GraphicList
	for _, name := range list {
		card, err := newGraphic(graphicSysfsDir, name)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func newGraphic(dir, name string) (*Graphic, error) {
	card, err := utils.NewCardInfo(dir, name)
	if err != nil {
		return nil, err
	}
	return &Graphic{
		CardInfo: *card,
	}, nil
}
