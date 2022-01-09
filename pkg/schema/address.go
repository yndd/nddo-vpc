package schema

import (
	"fmt"
	"strings"

	"inet.af/netaddr"
)

func (x *schema) CreateDeviceInterfaceSubInterfaceAddressInfo(ai *DeviceInterfaceSubInterfaceAddressInfo) {
	if d, ok := x.devices[*ai.DeviceName]; ok {
		if i, ok := d.interfaces[*ai.DeviceInterfaceName]; ok {
			if si, ok := i.subInterfaces[*ai.DeviceInterfaceSubInterfaceIndex]; ok {
				p := netaddr.MustParseIPPrefix(*ai.Prefix)
				if p.IP().Is4() {
					if subai, ok := si.ipv4[*ai.Prefix]; !ok {
						si.ipv4[*ai.Prefix] = &deviceInterfaceSubInterfaceAddressInfo{
							DeviceInterfaceSubInterfaceAddressInfo: ai,
							subinterface:                           si,
						}
					} else {
						subai.DeviceInterfaceSubInterfaceAddressInfo = ai
					}
					
				} else {
					if subai, ok := si.ipv6[*ai.Prefix]; !ok {
						si.ipv6[*ai.Prefix] = &deviceInterfaceSubInterfaceAddressInfo{
							DeviceInterfaceSubInterfaceAddressInfo: ai,
							subinterface:                           si,
						}
					} else {
						subai.DeviceInterfaceSubInterfaceAddressInfo = ai
					}					
				}
			}
		}
	}
}

type DeviceInterfaceSubInterfaceAddressInfo struct {
	// ParentDependencies
	DeviceName                       *string
	DeviceInterfaceName              *string
	DeviceInterfaceSubInterfaceIndex *uint32
	//Data
	*DeviceInterfaceSubInterfaceAddressData
}

type DeviceInterfaceSubInterfaceAddressData struct {
	Prefix       *string
	Cidr         *string
	Address      *string
	PrefixLength *uint32
}

type deviceInterfaceSubInterfaceAddressInfo struct {
	// Data
	*DeviceInterfaceSubInterfaceAddressInfo
	// Parent
	subinterface *deviceInterfaceSubInterface
}

func (x *deviceInterfaceSubInterfaceAddressInfo) Print(af, prefix string, n int) {
	fmt.Printf("%s Address IP Prefix %s: %s %s %s %s %d\n", strings.Repeat(" ", n), af, prefix, *x.Prefix, *x.Cidr, *x.Address, *x.PrefixLength)
}
