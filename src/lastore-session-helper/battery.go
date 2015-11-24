package main

import "dbus/com/deepin/daemon/power"
import log "github.com/cihub/seelog"

var MinBatteryPercent = 30.0

func (l *Lastore) MonitorBattery() {
	m, err := power.NewPower("com.deepin.daemon.Power", "/com/deepin/daemon/Power")
	if err != nil {
		log.Warnf("Failed MonitorBattery: %v\n", err)
	}

	l.updateBatteryInfo(m.BatteryPercentage.Get(), m.BatteryIsPresent.Get())

	m.BatteryIsPresent.ConnectChanged(func() {
		l.updateBatteryInfo(m.BatteryPercentage.Get(), m.BatteryIsPresent.Get())
	})

	m.BatteryPercentage.ConnectChanged(func() {
		l.updateBatteryInfo(m.BatteryPercentage.Get(), m.BatteryIsPresent.Get())
	})

}

func (l *Lastore) updateBatteryInfo(percent float64, present bool) {
	if !present {
		// power is online mode, clear notified flag
		l.notifiedBattery = false
		return
	}

	log.Infof("Current battery percentage:%v (notified:%v)\n", percent, l.notifiedBattery)
	if l.notifiedBattery {
		return
	}
	if percent < MinBatteryPercent && l.SystemOnChanging {
		l.notifiedBattery = true
		NotifyLowPower()
	}
}
