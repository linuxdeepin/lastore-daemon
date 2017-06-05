/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package system

import (
	log "github.com/cihub/seelog"
	"github.com/howeyc/fsnotify"
	"path"
	"sync"
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
				log.Warn(err)
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
