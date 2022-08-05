package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSystemInfoUtil(t *testing.T) {
	useDbus := NotUseDBus
	NotUseDBus = true
	defer func() {
		NotUseDBus = useDbus
	}()
	sys := getSystemInfo()
	assert.NotEmpty(t, sys.SystemName)
	assert.NotEmpty(t, sys.ProductType)
	assert.NotEmpty(t, sys.EditionName)
	assert.NotEmpty(t, sys.Version)
	assert.NotEmpty(t, sys.HardwareId)
	assert.NotEmpty(t, sys.Processor)
}
