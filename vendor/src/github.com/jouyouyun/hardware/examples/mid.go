package main

import (
	"fmt"

	"github.com/jouyouyun/hardware"
)

func main() {
	mid, err := hardware.GenMachineID()
	if err != nil {
		fmt.Println("Failed to generate machine id:", err)
		return
	}
	fmt.Println("Machine id:", mid)
}
