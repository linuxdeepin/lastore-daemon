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

func (l *Lastore) monitorBatteryPersent() {
	l.power.HasBattery.ConnectChanged(func() {
		if !l.power.HasBattery.Get() {
			l.notifiedBattery = false
		}
	})
}

func (l *Lastore) checkBattery() {
	if l.notifiedBattery {
		return
	}
	percent := l.power.BatteryPercentage.Get()
	if percent <= MinBatteryPercent && l.power.HasBattery.Get() {
		l.notifiedBattery = true
		NotifyLowPower()
	}
}
