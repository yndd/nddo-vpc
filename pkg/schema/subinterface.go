package schema

import (
	"fmt"
	"strconv"
	"strings"
)

func (x *schema) CreateDeviceInterfaceSubInterface(subi *DeviceInterfaceSubInterface) {
	var itfceSubItfce string
	if d, ok := x.GetDevices()[*subi.DeviceName]; ok {
		if i, ok := d.GetInterfaces()[*subi.DeviceInterfaceName]; ok {
			if si, ok := i.GetSubInterfaces()[*subi.Index]; !ok {
				i.GetSubInterfaces()[*si.GetSubInterface().Index] = NewNetworkSubInterface(i, subi)

				itfceSubItfce = strings.Join([]string{*i.GetInterface().Name, strconv.Itoa(int(*si.GetSubInterface().Index))}, ".")
			}

		}
	}
	if d, ok := x.devices[*subi.DeviceName]; ok {
		if ni, ok := d.GetNetworkInstances()[*subi.DeviceNetworkInstanceName]; ok {
			ni.GetSubInterfaces()[itfceSubItfce] = x.GetDevices()[*subi.DeviceName].GetInterfaces()[*subi.DeviceInterfaceName].GetSubInterfaces()[*subi.Index]

		}
	}
}

type NetworkSubInterface interface {
	GetIpv4AddressInfo() map[string]AddressInfo
	GetIpv6AddressInfo() map[string]AddressInfo
	GetNetworkInstance() NetworkInstance
	GetIndex() uint32
	GetKind() string
	GetOuterTag() uint32
	GetInnerTag() uint32
	GetSubInterface() *DeviceInterfaceSubInterface
	SetSubInterface(*DeviceInterfaceSubInterface)
	Print(string, int)
}

func NewNetworkSubInterface(i NetworkInterface, si *DeviceInterfaceSubInterface) NetworkSubInterface {
	return &deviceInterfaceSubInterface{
		DeviceInterfaceSubInterface: si,
		itfce:                       i,
		ipv4:                        make(map[string]AddressInfo),
		ipv6:                        make(map[string]AddressInfo),
	}
}

type DeviceInterfaceSubInterface struct {
	// ParentDependencies
	DeviceName                *string
	DeviceInterfaceName       *string
	DeviceNetworkInstanceName *string
	//Data
	*DeviceInterfaceSubInterfaceData
}

type DeviceInterfaceSubInterfaceData struct {
	Index    *uint32
	Neighbor NetworkSubInterface
	Tagging  *string
	Kind     *string
	OuterTag *uint32
	InnerTag *uint32
}

type deviceInterfaceSubInterface struct {
	*DeviceInterfaceSubInterface
	itfce NetworkInterface
	ni    NetworkInstance
	ipv4  map[string]AddressInfo
	ipv6  map[string]AddressInfo
}

func (x *deviceInterfaceSubInterface) GetIpv4AddressInfo() map[string]AddressInfo {
	return x.ipv4
}

func (x *deviceInterfaceSubInterface) GetIpv6AddressInfo() map[string]AddressInfo {
	return x.ipv6
}

func (x *deviceInterfaceSubInterface) GetInterface() NetworkInterface {
	return x.itfce
}

func (x *deviceInterfaceSubInterface) GetNetworkInstance() NetworkInstance {
	return x.ni
}

func (x *deviceInterfaceSubInterface) GetIndex() uint32 {
	return *x.Index
}

func (x *deviceInterfaceSubInterface) GetKind() string {
	return *x.Kind
}

func (x *deviceInterfaceSubInterface) GetOuterTag() uint32 {
	return *x.OuterTag
}

func (x *deviceInterfaceSubInterface) GetInnerTag() uint32 {
	return *x.InnerTag
}

func (x *deviceInterfaceSubInterface) GetNeighbor() NetworkSubInterface {
	return x.Neighbor
}

func (x *deviceInterfaceSubInterface) GetSubInterface() *DeviceInterfaceSubInterface {
	return x.DeviceInterfaceSubInterface
}

func (x *deviceInterfaceSubInterface) SetSubInterface(d *DeviceInterfaceSubInterface) {
	x.DeviceInterfaceSubInterface = d
}

func (x *deviceInterfaceSubInterface) Print(subItfceName string, n int) {
	fmt.Printf("%s SubInterface: %s Kind: %s Tagging: %s InterfaceSelecterKind: %s BdName: %s\n",
		strings.Repeat(" ", n), subItfceName, *x.Kind, *x.Tagging, *x.Kind, x.GetNetworkInstance().GetName())
	n++
	fmt.Printf("%s Local Addressing Info\n", strings.Repeat(" ", n))
	for prefix, i := range x.ipv4 {
		i.Print("ipv4", prefix, n)
	}
	for prefix, i := range x.ipv6 {
		i.Print("ipv6", prefix, n)
	}

	if x.Neighbor != nil {
		fmt.Printf("%s Neighbor Addressing Info\n", strings.Repeat(" ", n))
		for prefix, i := range x.Neighbor.GetIpv4AddressInfo() {
			i.Print(string("ipv4"), prefix, n)
		}
		for prefix, i := range x.Neighbor.GetIpv6AddressInfo() {
			i.Print(string("ipv6"), prefix, n)
		}
	}

}
