package utils

import (
	"os"
	"testing"
)

func TestRemoteFingerprint(t *testing.T) {
	if os.Getenv("NO_TEST_NETWORK") == "1" {
		t.Skip()
	}
	line, err := RemoteCatLine("http://packages.deepin.com/deepin/fixme/index")
	if err != nil {
		t.Fatal("E:", err)
	}
	if len(line) != 32 {
		t.Fatalf("%q not a md5sum value", line)
	}
}
