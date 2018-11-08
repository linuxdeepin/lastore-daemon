package main

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLastoreSmartmirrorDaemon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LastoreSmartmirrorDaemon Suite")
}
