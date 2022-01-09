package schema

import (
	"fmt"
	"strconv"
	"strings"
)

func (x *schema) CreateDeviceInterfaceSubInterface(subi *DeviceInterfaceSubInterface) {
	var itfceSubItfce string
	if d, ok := x.devices[*subi.DeviceName]; ok {
		if i, ok := d.interfaces[*subi.DeviceInterfaceName]; ok {
			if si, ok := i.subInterfaces[*subi.Index]; !ok {
				i.subInterfaces[*si.Index] = &deviceInterfaceSubInterface{
					DeviceInterfaceSubInterface: subi,
					itfce:                       i,
					ni:                          d.networkInstances[*si.DeviceNetworkInstanceName],
					ipv4:                        make(map[string]*deviceInterfaceSubInterfaceAddressInfo),
					ipv6:                        make(map[string]*deviceInterfaceSubInterfaceAddressInfo),
				}
				itfceSubItfce = strings.Join([]string{*i.Name, strconv.Itoa(int(*si.Index))}, ".")
			}

		}
	}
	if d, ok := x.devices[*subi.DeviceName]; ok {
		if ni, ok := d.networkInstances[*subi.DeviceNetworkInstanceName]; ok {
			ni.subInterfaces[itfceSubItfce] = x.devices[*subi.DeviceName].interfaces[*subi.DeviceInterfaceName].subInterfaces[*subi.Index]

		}
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
	Neighbor *deviceInterfaceSubInterface
	Tagging  *string
	Kind     *string
	OuterTag *uint32
	InnerTag *uint32
}

type deviceInterfaceSubInterface struct {
	*DeviceInterfaceSubInterface
	itfce *deviceInterface
	ni    *deviceNetworkInstance
	ipv4  map[string]*deviceInterfaceSubInterfaceAddressInfo
	ipv6  map[string]*deviceInterfaceSubInterfaceAddressInfo
}

func (x *deviceInterfaceSubInterface) Print(subItfceName string, n int) {
	fmt.Printf("%s SubInterface: %s Kind: %s Tagging: %s InterfaceSelecterKind: %s BdName: %s\n",
		strings.Repeat(" ", n), subItfceName, *x.Kind, *x.Tagging, *x.Kind, *x.ni.Name)
	n++
	fmt.Printf("%s Local Addressing Info\n", strings.Repeat(" ", n))
	for prefix, i := range x.ipv4 {
		i.Print("ipv4", prefix, n)
	}
	for prefix, i := range x.ipv6 {
		i.Print("ipv6", prefix, n)
	}
	/*
		if x.Neighbor != nil {
			fmt.Printf("%s Neighbor Addressing Info\n", strings.Repeat(" ", n))
			for prefix, i := range x.Neighbor.GetAddressesInfo(string(ipamv1alpha1.AddressFamilyIpv4)) {
				i.Print(string(ipamv1alpha1.AddressFamilyIpv4), prefix, n)
			}
			for prefix, i := range x.neighbor.GetAddressesInfo(string(ipamv1alpha1.AddressFamilyIpv6)) {
				i.Print(string(ipamv1alpha1.AddressFamilyIpv6), prefix, n)
			}
		}
	*/
}
