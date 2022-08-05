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
	assert.NotEmpty(t, sys)
}
