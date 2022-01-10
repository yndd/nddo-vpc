package schema

import (
	"fmt"
	"strings"
)

func (x *schema) CreateDevice(dev *Device) {
	if d, ok := x.devices[*dev.Name]; !ok {
		x.devices[d.GetName()] = NewNetworkDevice(dev)
	} else {
		d.SetDevice(dev)
	}

}

type NetworkDevice interface {
	GetInterfaces() map[string]NetworkInterface
	GetNetworkInstances() map[string]NetworkInstance
	GetName() string
	GetKind() string
	GetSwRelease() string
	GetPlatform() string
	GetDevice() *Device
	SetDevice(*Device)
	Print(string, int)
}

func NewNetworkDevice(d *Device) NetworkDevice {
	return &device{
		Device:           d,
		interfaces:       make(map[string]NetworkInterface),
		networkInstances: make(map[string]NetworkInstance),
	}
}

type Device struct {
	// Data
	Name     *string
	Index    *uint32
	Kind     *string
	SwRelease  *string
	Platform *string
}

type device struct {
	*Device
	interfaces       map[string]NetworkInterface
	networkInstances map[string]NetworkInstance
}

func (x *device) GetInterfaces() map[string]NetworkInterface {
	return x.interfaces
}

func (x *device) GetNetworkInstances() map[string]NetworkInstance {
	return x.networkInstances
}

func (x *device) GetName() string {
	return *x.Name
}

func (x *Device) GetIndex() uint32 {
	return *x.Index
}

func (x *Device) GetKind() string {
	return *x.Kind
}

func (x *Device) GetSwRelease() string {
	return *x.SwRelease
}

func (x *Device) GetPlatform() string {
	return *x.Platform
}

func (x *device) GetDevice() *Device {
	return x.Device
}

func (x *device) SetDevice(d *Device) {
	x.Device = d
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
