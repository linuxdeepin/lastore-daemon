package main

import (
	"internal/system"
	"io/ioutil"
	"strings"
)

type offlineManager struct {
}

func (m *offlineManager) check() error {

	return nil
}

// UpdateOfflineSourceFile repos: 离线仓库地址列表 单个地址eg:deb [trusted=yes] file:///home/lee/patch/temp/ eagle main
func (m *offlineManager) UpdateOfflineSourceFile(repos []string) error {
	return ioutil.WriteFile(system.OfflineSourceFile, []byte(strings.Join(repos, "\n")), 0644)
}

func (m *offlineManager) GetOfflineUpdateInfo() []string {
	return nil
}
