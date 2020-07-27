package network

// #cgo CFLAGS: -Wall -g
// #include "ip.h"
// #include <stdlib.h>
import "C"

import (
	"os"
	"path/filepath"
	"unsafe"

	"github.com/jouyouyun/hardware/utils"
)

const (
	netSysfsDir   = "/sys/class/net"
	netVirtualDir = "/sys/devices/virtual/net"
)

// Network store network info
type Network struct {
	utils.CardInfo

	Address string
	IP      string
}

// NetworkList network list
type NetworkList []*Network

// GetNetworkList return network card list
func GetNetworkList() (NetworkList, error) {
	var netList NetworkList
	ifaceList, _ := utils.ScanDir(netSysfsDir, filterIface)
	for _, iface := range ifaceList {
		net, err := newNetwork(netSysfsDir, iface)
		if err != nil {
			return nil, err
		}
		netList = append(netList, net)
	}
	return netList, nil
}

func newNetwork(dir, iface string) (*Network, error) {
	card, err := utils.NewCardInfo(dir, iface)
	if err != nil {
		return nil, err
	}
	var net = Network{CardInfo: *card}
	net.Address, _ = utils.ReadFileContent(filepath.Join(dir, iface, "address"))
	net.IP = getIfaceIP(iface)
	return &net, nil
}

func getIfaceIP(iface string) string {
	ciface := C.CString(iface)
	defer C.free(unsafe.Pointer(ciface))

	cret := C.get_iface_ip(ciface)
	defer C.free(unsafe.Pointer(cret))

	ret := C.GoString(cret)
	return ret
}

func filterIface(iface string) bool {
	return isVirtualIface(iface, netVirtualDir)
}

func isVirtualIface(iface, dir string) bool {
	_, err := os.Stat(filepath.Join(dir, iface))
	return err == nil || os.IsExist(err)
}
