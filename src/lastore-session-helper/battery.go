/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
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
