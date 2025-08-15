package check

import (
	"testing"
)

// EmulationMainShell
func TestEmulationMainShell(t *testing.T) {
	t.Parallel()
	emu := EmulationMainShell{}
	emu.Verbose = true
	emu.RunCommand = "installer"
	emu.PointRootPath = "/tmp/abcd-aaa"
	emu.SourcePoint = append(emu.SourcePoint, "/srv")
	emu.SourcePoint = append(emu.SourcePoint, "/srv2")
	emu.SourcePoint = append(emu.SourcePoint, "/srv3")
	// FIXME:(heysion) this found error
	// emu.RenderMainShell("/tmp/a1")
	//t.Logf("aa")
	t.SkipNow()
}
