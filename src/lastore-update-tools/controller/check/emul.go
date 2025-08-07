package check

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

// check pkg with overlayfs
func EmulationUpdate(CacheCfg *cache.CacheInfo) error {
	// fetch mountpoint with find
	callcmd := func(args ...string) ([]string, error) {
		outStream, err := runcmd.RunnerOutput(100, args[0], args[1:]...)
		if err != nil {
			log.Errorf("find failed : %v", err)
			return nil, err
		}
		log.Debugf("%+v", outStream)
		out := strings.Split(outStream, "\n")
		log.Debugf("%+v", out)
		return out, nil
	}
	// find / -maxdepth 1
	outStream, err := callcmd(strings.Split("find / -maxdepth 1", " ")...)
	if err != nil {
		log.Errorf("find failed : %v", err)
	}
	log.Debugf("%+v", outStream)

	// findmnt --real -r -o target -n
	outAppend, err := callcmd(strings.Split("findmnt --real -r -o target -n", " ")...)
	if err != nil {
		log.Errorf("findmnt failed : %v", err)
	}

	log.Debugf("%+v", outAppend)
	mergeOut := append(outStream, outAppend...)

	uniqueMap := make(map[string]bool)
	sourcePoint := []string{}
	for _, str := range mergeOut {
		if !uniqueMap[str] {
			uniqueMap[str] = true
			sourcePoint = append(sourcePoint, str)
		}
	}
	uniqueMap = nil
	mergeOut = nil
	outStream = nil
	outStream = nil

	// 按字符排序
	sort.Strings(sourcePoint)
	// log.Debugf("%+v", sourcePoint)

	// create mount point
	mountPointRootPath, err := ioutil.TempDir("/tmp/", "rootfs-point_")
	if err != nil {
		log.Errorf("create rootfs point failed %v", err)
	}
	log.Debugf("%+v", mountPointRootPath)

	// create mount point

	func() {
		func() {
			if err := fs.CreateDirMode(mountPointRootPath+"/upperdir", 0755); err != nil {
				log.Errorf("error creating mount point upperdir: %v", err)
			}

			if err := fs.CreateDirMode(mountPointRootPath+"/workdir", 0755); err != nil {
				log.Errorf("error creating mount point upperdir: %v", err)
			}

			if err := fs.CreateDirMode(mountPointRootPath+"/rootfs", 0755); err != nil {
				log.Errorf("error creating mount point upperdir: %v", err)
			}
		}()

		for _, spoint := range sourcePoint {
			if mode, err := fs.ReadMode(spoint); err != nil {
				continue
			} else {
				if err := fs.CreateDirMode(mountPointRootPath+"/upperdir/"+spoint, mode); err != nil {
					continue
				}
				if err := fs.CreateDirMode(mountPointRootPath+"/workdir/"+spoint, mode); err != nil {
					continue
				}
				if err := fs.CreateDirMode(mountPointRootPath+"/rootfs/"+spoint, mode); err != nil {
					continue
				}
			}
		}
	}()

	RunCommandPath, err := ioutil.TempFile("/tmp/", "output_")
	if err != nil {
		log.Errorf("Error creating temporary %+v", err)
	}

	emulation := EmulationMainShell{
		PointRootPath: mountPointRootPath,
		RunCommand:    RunCommandPath.Name(),
		SourcePoint: func() []string {
			filtered := []string{}
			for idx := range sourcePoint {
				switch sourcePoint[idx] {
				case "/":
				case "/dev":
				case "/proc":
				case "/lost+found":
				default:
					filtered = append(filtered, sourcePoint[idx])
				}
			}
			log.Debugf("Filtered: %v", filtered)
			return filtered
		}(),
	}

	emulation.Verbose = true
	log.Debugf("Emulation started %v", emulation)
	if outRenderPath, err := ioutil.TempFile("/tmp/", "output_"); err == nil {
		emulation.RenderMainShell(outRenderPath.Name())
		emulation.MainCommand = outRenderPath.Name()
		outRenderPath.Close()
	}

	emulation.DebInstallPath = CacheCfg.WorkStation + "/deb/"
	emulation.RenderDebInstallShell(emulation.RunCommand)
	RunCommandPath.Close()
	{
		emuRun, err := callcmd(strings.Split(fmt.Sprintf("/usr/bin/unshare --mount --pid --fork %s", emulation.MainCommand), " ")...)
		if err != nil {
			log.Errorf("find failed : %v", err)
			CacheCfg.InternalState.IsEmulationCheck = false
		} else {
			CacheCfg.InternalState.IsEmulationCheck = true
		}
		log.Debugf("%+v", emuRun)
	}
	return nil
}
