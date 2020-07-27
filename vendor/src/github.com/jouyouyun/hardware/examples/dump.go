package main

import (
	"encoding/json"
	"fmt"

	"github.com/jouyouyun/hardware/battery"
	"github.com/jouyouyun/hardware/bluetooth"
	"github.com/jouyouyun/hardware/camera"
	"github.com/jouyouyun/hardware/cpu"
	"github.com/jouyouyun/hardware/disk"
	"github.com/jouyouyun/hardware/dmi"
	"github.com/jouyouyun/hardware/graphic"
	"github.com/jouyouyun/hardware/memory"
	"github.com/jouyouyun/hardware/network"
	"github.com/jouyouyun/hardware/peripherals"
	"github.com/jouyouyun/hardware/sound"
)

func main() {
	dumpCPU()
	dumpMemory()
	dumpDMI()
	dumpDisk()
	dumpGraphic()
	dumpNetwork()
	dumpSound()
	dumpPeripherals()
	dumpBluetooth()
	dumpBattery()
	dumpCamera()
}

func dumpDMI() {
	doDump("dmi", func() (interface{}, error) {
		info, err := dmi.GetDMI()
		return info, err
	})
}

func dumpCPU() {
	doDump("cpu", func() (interface{}, error) {
		info, err := cpu.NewCPU()
		return info, err
	})
}

func dumpBattery() {
	doDump("battery", func() (interface{}, error) {
		info, err := battery.GetBatteryList()
		return info, err
	})
}

func dumpBluetooth() {
	doDump("bluetooth", func() (interface{}, error) {
		info, err := bluetooth.GetBluetoothList()
		return info, err
	})
}

func dumpCamera() {
	doDump("camera", func() (interface{}, error) {
		info, err := camera.GetCameraList()
		return info, err
	})
}

func dumpDisk() {
	doDump("disk", func() (interface{}, error) {
		info, err := disk.GetDiskList()
		return info, err
	})
}

func dumpNetwork() {
	doDump("network", func() (interface{}, error) {
		info, err := network.GetNetworkList()
		return info, err
	})
}

func dumpGraphic() {
	doDump("graphic", func() (interface{}, error) {
		info, err := graphic.GetGraphicList()
		return info, err
	})
}

func dumpMemory() {
	doDump("memory", func() (interface{}, error) {
		info, err := memory.GetMemoryList()
		return info, err
	})
}

func dumpPeripherals() {
	doDump("peripherals", func() (interface{}, error) {
		info, err := peripherals.GetPeripheralsList()
		return info, err
	})
}

func dumpSound() {
	doDump("sound", func() (interface{}, error) {
		info, err := sound.GetSoundList()
		return info, err
	})
}

func doDump(name string, getter func() (interface{}, error)) {
	fmt.Printf("Dump %s: [START]\n", name)
	defer fmt.Printf("Dump %s: [DONE]\n\n", name)
	info, err := getter()
	if err != nil {
		fmt.Printf("Failed to get %s: %s\n", name, err)
		return
	}
	data, err := json.Marshal(info)
	if err != nil {
		fmt.Printf("Failed to marshal %s: %s\n", name, err)
		return
	}
	fmt.Println("\t", string(data))
}
