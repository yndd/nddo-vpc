package schema

import (
	"fmt"
	"strings"

	"inet.af/netaddr"
)

func (x *schema) CreateDeviceInterfaceSubInterfaceAddressInfo(ai *DeviceInterfaceSubInterfaceAddressInfo) {
	if d, ok := x.devices[*ai.DeviceName]; ok {
		if i, ok := d.GetInterfaces()[*ai.DeviceInterfaceName]; ok {
			if si, ok := i.GetSubInterfaces()[*ai.DeviceInterfaceSubInterfaceIndex]; ok {
				p := netaddr.MustParseIPPrefix(*ai.Prefix)
				if p.IP().Is4() {
					if subai, ok := si.GetIpv4AddressInfo()[*ai.Prefix]; !ok {
						si.GetIpv4AddressInfo()[*ai.Prefix] = NewAddressInfo(si, ai)
					} else {
						subai.SetAddressInfo(ai)
					}

				} else {
					if subai, ok := si.GetIpv6AddressInfo()[*ai.Prefix]; !ok {
						si.GetIpv6AddressInfo()[*ai.Prefix] = NewAddressInfo(si, ai)
					} else {
						subai.SetAddressInfo(ai)
					}
				}
			}
		}
	}
}

type AddressInfo interface {
	GetAddressInfo() *DeviceInterfaceSubInterfaceAddressInfo
	SetAddressInfo(*DeviceInterfaceSubInterfaceAddressInfo)
	GetPrefix() string
	GetCidr() string
	GetAddress() string
	GetPrefixLength() uint32
	Print(string, string, int)
}

func NewAddressInfo(si NetworkSubInterface, ai *DeviceInterfaceSubInterfaceAddressInfo) AddressInfo {
	return &deviceInterfaceSubInterfaceAddressInfo{
		DeviceInterfaceSubInterfaceAddressInfo: ai,
		subinterface:                           si,
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
	subinterface NetworkSubInterface
}

func (x *deviceInterfaceSubInterfaceAddressInfo) GetAddressInfo() *DeviceInterfaceSubInterfaceAddressInfo {
	return x.DeviceInterfaceSubInterfaceAddressInfo
}

func (x *deviceInterfaceSubInterfaceAddressInfo) SetAddressInfo(ai *DeviceInterfaceSubInterfaceAddressInfo) {
	x.DeviceInterfaceSubInterfaceAddressInfo = ai
}

func (x *deviceInterfaceSubInterfaceAddressInfo) GetPrefix() string {
	return *x.Prefix
}

func (x *deviceInterfaceSubInterfaceAddressInfo) GetCidr() string {
	return *x.Cidr
}

func (x *deviceInterfaceSubInterfaceAddressInfo) GetAddress() string {
	return *x.Address
}

func (x *deviceInterfaceSubInterfaceAddressInfo) GetPrefixLength() uint32 {
	return *x.PrefixLength
}

func (x *deviceInterfaceSubInterfaceAddressInfo) Print(af, prefix string, n int) {
	fmt.Printf("%s Address IP Prefix %s: %s %s %s %s %d\n", strings.Repeat(" ", n), af, prefix, *x.Prefix, *x.Cidr, *x.Address, *x.PrefixLength)
}
