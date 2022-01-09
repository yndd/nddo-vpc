package schema

import (
	"fmt"
	"strings"
)

func (x *schema) CreateDevice(dev *Device) {
	if d, ok := x.devices[*dev.Name]; !ok {
		x.devices[*d.Name] = &device{
			Device:           dev,
			interfaces:       make(map[string]*deviceInterface),
			networkInstances: make(map[string]*deviceNetworkInstance),
		}
	} else {
		d.Device = dev
	}
	
}

type Device struct {
	// Data
	Name     *string
	Index    *uint32
	Kind     *string
	Release  *string
	Platform *string
}

type device struct {
	*Device
	interfaces       map[string]*deviceInterface
	networkInstances map[string]*deviceNetworkInstance
}

func (x *device) Print(deviceName string, n int) {
	fmt.Printf("%s Node Name: %s Kind: %s\n", strings.Repeat(" ", n), deviceName, *x.Kind)
	n++
	for itfceName, i := range x.interfaces {
		i.Print(itfceName, n)
	}
	for niName, ni := range x.networkInstances {
		ni.Print(niName, n)
	}
}
