package main

import log "github.com/cihub/seelog"
import "pkg.deepin.io/lib/gettext"
import "dbus/org/freedesktop/notifications"
import "fmt"
import "os/exec"

type Action struct {
	Id       string
	Name     string
	Callback func()
}

func SendNotify(msg string, actions []Action) {
	log.Infof("Notify: %q, %v\n", msg, actions)
	n, err := notifications.NewNotifier("org.freedesktop.Notifications", "/org/freedesktop/Notifications")

	var as []string
	for _, action := range actions {
		as = append(as, action.Id, action.Name)
	}

	id, err := n.Notify("appstore", 0, "deepin-appstore", "", msg, as, nil, -1)
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
	} else {
		msg = fmt.Sprintf(gettext.Tr("%q failed to install."), pkgId)
	}
	SendNotify(msg, ac)
}

func NotifyRemove(pkgId string, succeed bool, ac []Action) {
	var msg string
	if succeed {
		msg = fmt.Sprintf(gettext.Tr("%q removed successfully."), pkgId)
	} else {
		msg = fmt.Sprintf(gettext.Tr("%q failed to remove."), pkgId)
	}
	SendNotify(msg, ac)
}

// NotifyDownload send desktop notify for download job
func NotifyFailedDownload(pkgName string, ac []Action) {
	msg := fmt.Sprintf(gettext.Tr("%q failed to download."), pkgName)
	SendNotify(msg, ac)
}

//NotifyLowPower send notify for low power
func NotifyLowPower() {
	msg := gettext.Tr("In order to prevent automatic shutdown, please plug in for normal update.")
	SendNotify(msg, nil)
}

func NotifyUpgrade(succeed bool, ac []Action) {
	var msg string
	if succeed {
		// TODO: we need check this for any system change actions
		if FileExist("/var/run/reboot-required.pkgs") {
			msg = gettext.Tr("Updated successfully!") + " " + gettext.Tr("Some functions will take effect after rebooting.")
		} else {
			msg = gettext.Tr("Updated successfully!")
		}
	} else {
		msg = gettext.Tr("Failed to update.")
	}

	SendNotify(msg, ac)
}

func LaunchDCC(moduleName string) {
	cmd := exec.Command("dde-control-center", moduleName)
	cmd.Start()
	go cmd.Wait()
}

func NotifyNewUpdates(n int) {
	if n <= 0 {
		return
	}
	msg := fmt.Sprintf(gettext.Tr("%d application(s) need to be updated."), n)
	SendNotify(msg, []Action{Action{
		Id:   "update",
		Name: gettext.Tr("Update Now"),
		Callback: func() {
			LaunchDCC("system_info")
		},
	}})
}
