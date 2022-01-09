package schema

import (
	"fmt"
	"strconv"
	"strings"
)

func (x *schema) CreateDeviceInterface(di *DeviceInterface) {
	if d, ok := x.devices[*di.DeviceName]; ok {
		if i, ok := d.interfaces[*di.Name]; !ok {
			d.interfaces[*di.Name] = &deviceInterface{
				DeviceInterface: di,
				device:          d,
				subInterfaces:   make(map[uint32]*deviceInterfaceSubInterface),
			}
		} else {
			i.DeviceInterface = di
		}

	}
}

type DeviceInterface struct {
	// ParentDependencies
	DeviceName *string
	// Data
	*DeviceInterfaceData
}

type DeviceInterfaceData struct {
	Name         *string
	Kind         *string
	Lag          *bool
	LagMember    *bool
	LagName      *string
	Lacp         *bool
	LacpFallback *bool
}

type deviceInterface struct {
	*DeviceInterface
	device        *device
	subInterfaces map[uint32]*deviceInterfaceSubInterface
}

func (x *deviceInterface) Print(itfceName string, n int) {
	fmt.Printf("%s Interface: %s Kind: %s LAG: %t, LAG Member: %t\n", strings.Repeat(" ", n), itfceName, *x.Kind, *x.Lag, *x.LagMember)
	n++
	for subItfceName, i := range x.subInterfaces {
		i.Print(strconv.Itoa(int(subItfceName)), n)
	}
}
