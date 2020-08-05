/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package system

import (
	"path"
	"sync"

	log "github.com/cihub/seelog"
	"github.com/howeyc/fsnotify"
)

type DirMonitorChangeType string

type DirMonitor struct {
	sync.Mutex
	done    chan bool
	watcher *fsnotify.Watcher

	callbacks map[string]DirMonitorCallback
	baseDir   string
}

type DirMonitorCallback func(fpath string)

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

	err = f.watcher.Watch(f.baseDir)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event := <-f.watcher.Event:
				f.tryNotify(event)
			case err := <-f.watcher.Error:
				_ = log.Warn(err)
			case <-f.done:
				goto end
			}
		}
	end:
	}()
	return nil
}

func (f *DirMonitor) tryNotify(event *fsnotify.FileEvent) {
	f.Lock()
	defer f.Unlock()

	fpath := event.Name
	fn, ok := f.callbacks[fpath]
	if !ok {
		return
	}

	if event.IsModify() || event.IsDelete() {
		fn(fpath)
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
