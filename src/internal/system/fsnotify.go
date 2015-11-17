package system

import (
	log "github.com/cihub/seelog"
	"gopkg.in/fsnotify.v1"
	"path"
	"sync"
)

type DirMonitorChangeType string

const ()

type DirMonitor struct {
	sync.Mutex
	done    chan bool
	watcher *fsnotify.Watcher

	callbacks map[string]DirMonitorCallback
	baseDir   string
}

type DirMonitorCallback func(fpath string, ops uint32)

func (f *DirMonitor) Add(fn DirMonitorCallback, names ...string) error {
	f.Lock()
	for _, name := range names {
		fpath := path.Join(f.baseDir, name)
		if _, ok := f.callbacks[fpath]; ok {
			return ResourceExitError
		}
		f.callbacks[fpath] = fn
	}
	f.Unlock()
	return nil
}

func NewDirMonitor(baseDir string) *DirMonitor {
	return &DirMonitor{
		baseDir:   baseDir,
		done:      make(chan bool),
		callbacks: make(map[string]DirMonitorCallback),
	}
}

func (f *DirMonitor) Start() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	f.Lock()
	if f.watcher != nil {
		f.watcher.Close()
	}
	f.watcher = w
	f.Unlock()

	err = f.watcher.Add(f.baseDir)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event := <-f.watcher.Events:
				f.tryNotify(event.Name, event.Op)
			case err := <-f.watcher.Errors:
				log.Warn(err)
			case <-f.done:
				goto end
			}
		}
	end:
	}()
	return nil
}

func (f *DirMonitor) tryNotify(fpath string, op fsnotify.Op) {
	f.Lock()
	defer f.Unlock()

	fn, ok := f.callbacks[fpath]
	if !ok {
		return
	}

	if op&fsnotify.Write == fsnotify.Write || op&fsnotify.Remove == fsnotify.Remove {
		fn(fpath, uint32(op))
	}
}

func (f *DirMonitor) Stop() {
	f.done <- true

	f.Lock()
	if f.watcher != nil {
		f.watcher.Close()
		f.watcher = nil
	}
	f.Unlock()
}
