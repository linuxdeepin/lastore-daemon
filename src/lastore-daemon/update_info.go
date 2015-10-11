package main

import "pkg.deepin.io/lib/dbus"

func (m *Manager) refreshUpgradableApps() {
	infos := m.b.UpgradeInfo()
	changed := len(infos) != len(m.UpgradableApps)
	var apps []string
	for i, info := range infos {
		apps = append(apps, info.Package)
		if !changed && info != m.upgradableInfos[i] {
			changed = true
		}
	}

	if changed {
		m.UpgradableApps = apps
		m.upgradableInfos = infos
		dbus.NotifyChange(m, "UpgradableApps")
	}
}

func (m *Manager) PackageUpgradableInfo(pkgId string) (string, string, string) {
	for _, info := range m.upgradableInfos {
		if info.Package == pkgId {
			return info.CurrentVersion, info.LastVersion, info.ChangeLog
		}
	}
	return "", "", ""
}
