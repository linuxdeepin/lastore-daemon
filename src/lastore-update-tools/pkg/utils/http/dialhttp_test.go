package http

import (
	"testing"
)

var testDataSet1 = []struct {
	in  string
	out bool
}{
        {"xxxxs://example.com/testapp", true},
        {"xxxxs://example.com/testapp", false},
}

// DialUrlHttpGet
func TestDialUrlHttpGet(t *testing.T) {
	t.Parallel()
	for _, tds := range testDataSet1 {
		ret := DialUrlHttpGet(tds.in, 5)
		retB := (ret == nil)
		if retB != tds.out {
			t.Errorf("the key %v , ret %v", tds, ret)
		}
	}
}

var testDataSet2 = []struct {
	in    string
	out   string
	state bool
}{
        {"xxxxs://example.com/testapp", "Packages.gz", true},
        {"xxxxs://example.com/testapp", "binary-amd64", true},
        {"xxxxs://example.com/testapp/AAAA", "AAAA", false},
}

// DownloadFileHttpGet
func TestDownloadFileHttpGet(t *testing.T) {
	t.Parallel()
	for _, tds := range testDataSet2 {
		out, _, ret := DownloadFileHttpGet(tds.in, "/tmp/", 10)
		retB := (ret == nil)
		//t.Logf("DownloadFileHttpGet out:%+v,ret:%+v,in:%+v", out, ret, tds)
		if retB != tds.state && (out == tds.out || !tds.state) {
			t.Errorf("the key %v , ret %v", tds, ret)
		}
	}
}
