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
	if l.power.HasBattery.Get() &&
		l.power.OnBattery.Get() &&
		percent <= MinBatteryPercent {
		l.notifiedBattery = true
		NotifyLowPower()
	}
}
