package schema

import (
	"fmt"
	"strconv"
	"strings"
)

func (x *schema) CreateDeviceInterface(di *DeviceInterface) {
	if d, ok := x.devices[*di.DeviceName]; ok {
		if i, ok := d.GetInterfaces()[*di.Name]; !ok {
			d.GetInterfaces()[*di.Name] = NewNetworkInterface(d, i.GetInterface())
		} else {
			i.SetInterface(di)
		}
	}
}

type NetworkInterface interface {
	GetSubInterfaces() map[uint32]NetworkSubInterface
	GetName() string
	GetInterface() *DeviceInterface
	SetInterface(*DeviceInterface)
	Print(string, int)
}

func NewNetworkInterface(d NetworkDevice, di *DeviceInterface) NetworkInterface {
	return &deviceInterface{
		DeviceInterface: di,
		device:          d,
		subInterfaces:   make(map[uint32]NetworkSubInterface),
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
	device        NetworkDevice
	subInterfaces map[uint32]NetworkSubInterface
}

func (x *deviceInterface) GetSubInterfaces() map[uint32]NetworkSubInterface {
	return x.subInterfaces
}

func (x *deviceInterface) GetName() string {
	return *x.Name
}

func (x *deviceInterface) GetInterface() *DeviceInterface {
	return x.DeviceInterface
}

func (x *deviceInterface) SetInterface(d *DeviceInterface) {
	x.DeviceInterface = d
}

func (x *deviceInterface) Print(itfceName string, n int) {
	fmt.Printf("%s Interface: %s Kind: %s LAG: %t, LAG Member: %t\n", strings.Repeat(" ", n), itfceName, *x.Kind, *x.Lag, *x.LagMember)
	n++
	for subItfceName, i := range x.subInterfaces {
		i.Print(strconv.Itoa(int(subItfceName)), n)
	}
}
