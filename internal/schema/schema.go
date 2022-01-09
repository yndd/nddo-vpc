package schema

import (
	"fmt"
)

type SchemaOption func(*schema)

func NewSchema(opts ...SchemaOption) Schema {
	i := &schema{
		devices: make(map[string]*device),
		//links: make(map[string]Link),
		//nis:   make(map[string]Ni),
	}

	for _, f := range opts {
		f(i)
	}

	return i
}

var _ Schema = &schema{}

type Schema interface {
	CreateDevice(*Device)
	CreateDeviceInterface(*DeviceInterface)
	CreateDeviceNetworkInstance(*DeviceNetworkInstance)
	CreateDeviceInterfaceSubInterface(*DeviceInterfaceSubInterface)
	CreateDeviceInterfaceSubInterfaceAddressInfo(*DeviceInterfaceSubInterfaceAddressInfo)
	PrintDevices(string)
}

type schema struct {
	devices map[string]*device
}

func (x *schema) PrintDevices(crName string) {
	fmt.Printf("infrastructure node information: %s\n", crName)
	for deviceName, d := range x.devices {
		d.Print(deviceName, 1)
	}
}
