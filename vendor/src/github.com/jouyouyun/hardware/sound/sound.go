package sound

import (
	"github.com/jouyouyun/hardware/utils"
)

const (
	soundSysfsDir = "/sys/class/sound"
)

// Sound store sound card info
type Sound struct {
	utils.CardInfo
}

// SoundList sound card list
type SoundList []*Sound

// GetSoundList return sound card list
func GetSoundList() (SoundList, error) {
	list, _ := utils.ScanDir(soundSysfsDir, utils.FilterCardName)
	var cards SoundList
	for _, name := range list {
		card, err := newSound(soundSysfsDir, name)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func newSound(dir, name string) (*Sound, error) {
	card, err := utils.NewCardInfo(dir, name)
	if err != nil {
		return nil, err
	}
	return &Sound{CardInfo: *card}, nil
}
