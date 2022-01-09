package schema

import (
	"fmt"
	"strings"
)

func (x *schema) CreateDeviceNetworkInstance(dni *DeviceNetworkInstance) {
	if d, ok := x.devices[*dni.DeviceName]; ok {
		if ni, ok := d.networkInstances[*dni.Name]; !ok {
			d.networkInstances[*dni.Name] = &deviceNetworkInstance{
				DeviceNetworkInstance: dni,
				device:                d,
				subInterfaces:         make(map[string]*deviceInterfaceSubInterface),
			}
		} else {
			ni.DeviceNetworkInstance = dni
		}

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
	device        *device
	subInterfaces map[string]*deviceInterfaceSubInterface
}

func (x *deviceNetworkInstance) Print(niName string, n int) {
	fmt.Printf("%s Ni Name: %s Kind: %s\n", strings.Repeat(" ", n), niName, *x.Kind)
	n++
	for siName, i := range x.subInterfaces {
		i.Print(siName, n)
	}
}
