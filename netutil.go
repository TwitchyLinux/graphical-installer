package main

import (
	"fmt"
	"net"
	"os"
)

func readNetInfo(mw *mainWindow) {
	mw.setDebugValue([]string{"network interfaces"}, "")
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Fprintf(os.Stderr, "net.Interfaces() failed: %v", err)
		return
	}

	for _, intf := range ifaces {
		if intf.Name == "lo" {
			continue
		}
		mw.setDebugValue([]string{"network interfaces", intf.Name}, "")
		mw.setDebugValue([]string{"network interfaces", intf.Name, "Index"}, fmt.Sprint(intf.Index))
		if intf.HardwareAddr != nil {
			mw.setDebugValue([]string{"network interfaces", intf.Name, "MAC"}, intf.HardwareAddr.String())
		}
		mw.setDebugValue([]string{"network interfaces", intf.Name, "MTU"}, fmt.Sprint(intf.MTU))
		mw.setDebugValue([]string{"network interfaces", intf.Name, "Flags"}, intf.Flags.String())

		addrs, err := intf.Addrs()
		if err != nil {
			mw.setDebugValue([]string{"network interfaces", intf.Name, "address-error"}, err.Error())
			continue
		}
		for i, a := range addrs {
			mw.setDebugValue([]string{"network interfaces", intf.Name, fmt.Sprintf("address-%d", i)}, "")
			switch v := a.(type) {
			case *net.IPAddr:
				mw.setDebugValue([]string{"network interfaces", intf.Name, fmt.Sprintf("address-%d", i), "Type"}, "IP layer")
				mw.setDebugValue([]string{"network interfaces", intf.Name, fmt.Sprintf("address-%d", i), "IP"}, v.String())
			default:
				mw.setDebugValue([]string{"network interfaces", intf.Name, fmt.Sprintf("address-%d", i), "Network"}, v.Network())
				mw.setDebugValue([]string{"network interfaces", intf.Name, fmt.Sprintf("address-%d", i), "Details"}, v.String())
			}
		}
	}
}
