package schema

import (
	"fmt"
	"strings"
)

func (x *schema) CreateDeviceNetworkInstance(dni *DeviceNetworkInstance) {
	if d, ok := x.devices[*dni.DeviceName]; ok {
		if ni, ok := d.GetNetworkInstances()[*dni.Name]; !ok {
			d.GetNetworkInstances()[*dni.Name] = NewNetworkInstance(d, ni.GetNetworkInstance())
		} else {
			ni.SetNetworkInstance(dni)
		}

	}
}

type NetworkInstance interface {
	GetSubInterfaces() map[string]NetworkSubInterface
	GetName() string
	GetNetworkInstance() *DeviceNetworkInstance
	SetNetworkInstance(*DeviceNetworkInstance)
	Print(string, int)
}

func NewNetworkInstance(d NetworkDevice, dni *DeviceNetworkInstance) NetworkInstance {
	return &deviceNetworkInstance{
		DeviceNetworkInstance:  dni,
		device: d,
		subInterfaces: make(map[string]NetworkSubInterface),
	}
}

type DeviceNetworkInstance struct {
	// ParentDependencies
	DeviceName *string
	// Data
	*DeviceNetworkInstanceData
}

type DeviceNetworkInstanceData struct {
	Name  *string
	Index *uint32
	Kind  *string
}

type deviceNetworkInstance struct {
	*DeviceNetworkInstance
	device        NetworkDevice
	subInterfaces map[string]NetworkSubInterface
}

func (x *deviceNetworkInstance) GetSubInterfaces() map[string]NetworkSubInterface {
	return x.subInterfaces
}

func (x *deviceNetworkInstance) GetName() string {
	return *x.Name
}

func (x *deviceNetworkInstance) GetNetworkInstance() *DeviceNetworkInstance {
	return x.DeviceNetworkInstance
}

func (x *deviceNetworkInstance) SetNetworkInstance(d *DeviceNetworkInstance)  {
	x.DeviceNetworkInstance = d
}

func (x *deviceNetworkInstance) Print(niName string, n int) {
	fmt.Printf("%s Ni Name: %s Kind: %s\n", strings.Repeat(" ", n), niName, *x.Kind)
	n++
	for siName, i := range x.subInterfaces {
		i.Print(siName, n)
	}
}
