package update

type ChangelogInfo struct {
	Abstract string `json:"abstract,omitempty" yaml:"abstract,omitempty"` // abstract
	Section  []struct {
		Title string `json:"title,omitempty" yaml:"title,omitempty"` // title
		Notes string `json:"notes,omitempty" yaml:"notes,omitempty"` // notes
	}
	Length uint16 `json:"length,omitempty" yaml:"length,omitempty"` // length
}
