package main

import log "github.com/cihub/seelog"
import "pkg.deepin.io/lib/gettext"
import "dbus/org/freedesktop/notifications"
import "fmt"

type Action struct {
	Id       string
	Name     string
	Callback func()
}

func SendNotify(icon string, msg string, actions []Action) {
	log.Infof("Notify: %q, %v\n", msg, actions)
	n, err := notifications.NewNotifier("org.freedesktop.Notifications", "/org/freedesktop/Notifications")

	var as []string
	for _, action := range actions {
		as = append(as, action.Id, action.Name)
	}

	if icon == "" {
		icon = "deepin-appstore"
	}
	id, err := n.Notify("appstore", 0, icon, "", msg, as, nil, -1)
	if err != nil {
		log.Warnf("Notify failed: %q: %v\n", msg, err)
		return
	}

	n.ConnectNotificationClosed(func(_id uint32, reason uint32) {
		if id == id {
			notifications.DestroyNotifier(n)
		}
	})
	n.ConnectActionInvoked(func(_id uint32, actionId string) {
		if id != _id {
			return
		}
		for _, action := range actions {
			if action.Id == actionId && action.Callback != nil {
				action.Callback()
			}
		}
	})
}

// NotifyInstall send desktop notify for install job
func NotifyInstall(pkgId string, succeed bool, ac []Action) {
	var msg string
	if succeed {
		msg = fmt.Sprintf(gettext.Tr("%q installed successfully."), pkgId)
		SendNotify("package_install_succeed", msg, ac)
	} else {
		msg = fmt.Sprintf(gettext.Tr("%q failed to install."), pkgId)
		SendNotify("package_install_failed", msg, ac)
	}

}

func NotifyRemove(pkgId string, succeed bool, ac []Action) {
	var msg string
	if succeed {
		msg = fmt.Sprintf(gettext.Tr("%q removed successfully."), pkgId)
	} else {
		msg = fmt.Sprintf(gettext.Tr("%q failed to remove."), pkgId)
	}
	SendNotify("deepin-appstore", msg, ac)
}

// NotifyDownload send desktop notify for download job
func NotifyFailedDownload(pkgName string, ac []Action) {
	msg := fmt.Sprintf(gettext.Tr("%q failed to download."), pkgName)
	SendNotify("package_download_failed", msg, ac)
}

//NotifyLowPower send notify for low power
func NotifyLowPower() {
	msg := gettext.Tr("In order to prevent automatic shutdown, please plug in for normal update.")
	SendNotify("notification-battery_low", msg, nil)
}

func NotifyUpgrade(succeed bool, ac []Action) {
	var msg string
	if succeed {
		if FileExist("/var/run/reboot-required.pkgs") {
			msg = gettext.Tr("Updated successfully!") + " " + gettext.Tr("Some functions will take effect after rebooting.")
		} else {
			msg = gettext.Tr("Updated successfully!")
		}
		SendNotify("package_install_succeed", msg, ac)
	} else {
		msg = gettext.Tr("Failed to update.")
		SendNotify("package_update_failed", msg, ac)
	}

}

func NotifyNewUpdates(nApps int, hasLibs bool) {
	var msg string
	switch {
	case nApps > 0 && !hasLibs:
		msg = fmt.Sprintf(gettext.Tr("%d software need to be updated."), nApps)
	case nApps == 0 && hasLibs:
		msg = fmt.Sprintf(gettext.Tr("Some patches need to be updated."))
	case nApps > 0 && hasLibs:
		msg = fmt.Sprintf(gettext.Tr("Some patches and %d software need to be updated."), nApps)
	default:
		return
	}

	SendNotify("system_updated",
		msg, []Action{Action{
			Id:   "update",
			Name: gettext.Tr("Update Now"),
			Callback: func() {
				LaunchDCCAndUpgrade()
			},
		}})
}
