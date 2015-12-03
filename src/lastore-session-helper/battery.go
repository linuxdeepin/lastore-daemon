package main

var MinBatteryPercent = 30.0

func (l *Lastore) MonitorBatteryPersent() {
	l.upower.BatteryIsPresent.ConnectChanged(func() {
		if !l.upower.BatteryIsPresent.Get() {
			l.notifiedBattery = false
		}
	})
}

func (l *Lastore) checkBattery() {
	if l.notifiedBattery || !l.SystemOnChanging {
		return
	}
	percent := l.upower.BatteryPercentage.Get()
	if percent <= MinBatteryPercent && l.upower.BatteryIsPresent.Get() {
		l.notifiedBattery = true
		NotifyLowPower()
	}
}
