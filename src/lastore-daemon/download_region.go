package main

import (
	"fmt"
)

func (m *Manager) SetRegion(region string) error {
	if region != "mainland" && region != "international" {
		return fmt.Errorf("the region of %q is not supported", region)
	}
	return nil
}
