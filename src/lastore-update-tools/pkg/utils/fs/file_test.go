package fs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
)

func TestCheckFileExistState(t *testing.T) {
	file, err := ioutil.TempFile("/tmp/", "sha256_")
	if err != nil {
		log.Fatalf("Could not create temporary file %v", err)
	}
	defer os.Remove(file.Name())

	file.WriteString("1")

	file.Close()

	//filepath.Glob("/tmp/sha256_*")

	if rs, err := filepath.Glob("/tmp/sha256_*"); err != nil || len(rs) <= 0 {
		t.Error("failed: ", err, file.Name())
	} else {
		t.Logf("rs:%v", rs)
	}
}
