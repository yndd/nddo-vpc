package schema

import (
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"

	"github.com/yndd/ndd-runtime/pkg/utils"
	nddov1 "github.com/yndd/nddo-runtime/apis/common/v1"
)

type SchemaOption func(*schema)

func NewSchema(opts ...SchemaOption) Schema {
	i := &schema{
		devices: make(map[string]NetworkDevice),
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
	GetDevices() map[string]NetworkDevice
	PrintDevices(string)

	PopulateSchema(deviceName string, itfceInfo ItfceInfo, niInfo *DeviceNetworkInstanceData, addressAllocationStrategy *nddov1.AddressAllocationStrategy) error

	
}

type schema struct {
	devices map[string]NetworkDevice
}

func (x *schema) GetDevices() map[string]NetworkDevice {
	return x.devices
}

func (x *schema) PrintDevices(crName string) {
	fmt.Printf("infrastructure node information: %s\n", crName)
	for deviceName, d := range x.devices {
		d.Print(deviceName, 1)
	}
}

func (x *schema) PopulateSchema(deviceName string, itfceInfo ItfceInfo, niInfo *DeviceNetworkInstanceData, addressAllocationStrategy *nddov1.AddressAllocationStrategy) error {
	x.CreateDevice(&Device{
		Name: &deviceName,
	})

	x.CreateDeviceInterface(&DeviceInterface{
		DeviceName: &deviceName,
		DeviceInterfaceData: &DeviceInterfaceData{
			Name: utils.StringPtr(itfceInfo.GetItfceName()),
		},
	})

	var si *DeviceInterfaceSubInterfaceData
	if itfceInfo.GetItfceKind() == "interface" {
		si = &DeviceInterfaceSubInterfaceData{
			Kind: utils.StringPtr(itfceInfo.GetItfceKind()),
			//Neighbor:      // not relevant
			Index:    utils.Uint32Ptr(itfceInfo.GetVlanId()),
			OuterTag: utils.Uint32Ptr(itfceInfo.GetVlanId()), // user defined, what to do when user is not defining them?
			InnerTag: utils.Uint32Ptr(itfceInfo.GetVlanId()), // user defined, what to do when user is not defining them?

		}
	} else {
		si = &DeviceInterfaceSubInterfaceData{
			Kind: utils.StringPtr("vxlan"),
			//Neighbor:      // not relevant
			Index: utils.Uint32Ptr(itfceInfo.GetItfceIndex()),
			//OuterTag: utils.Uint32Ptr(itfceInfo.GetVlanId()), // user defined, what to do when user is not defining them?
			//InnerTag: utils.Uint32Ptr(itfceInfo.GetVlanId()), // user defined, what to do when user is not defining them?
		}
	}

	x.CreateDeviceInterfaceSubInterface(&DeviceInterfaceSubInterface{
		DeviceName:                      &deviceName,
		DeviceInterfaceName:             utils.StringPtr(itfceInfo.GetItfceName()),
		DeviceNetworkInstanceName:       utils.StringPtr(strings.Join([]string{*niInfo.Name, *niInfo.Kind}, "-")),
		DeviceInterfaceSubInterfaceData: si,
	})

	x.CreateDeviceNetworkInstance(&DeviceNetworkInstance{
		DeviceName: &deviceName,
		DeviceNetworkInstanceData: &DeviceNetworkInstanceData{
			Name:  utils.StringPtr(strings.Join([]string{*niInfo.Name, *niInfo.Kind}, "-")),
			Kind:  utils.StringPtr(*niInfo.Kind),
			Index: utils.Uint32Ptr(*niInfo.Index), // to be allocated
		},
	})

	if len(itfceInfo.GetIpv4Prefixes()) > 0 {
		for _, prefix := range itfceInfo.GetIpv4Prefixes() {
			ad, err := getAdressInfo(prefix, addressAllocationStrategy)
			if err != nil {
				return err
			}

			x.CreateDeviceInterfaceSubInterfaceAddressInfo((&DeviceInterfaceSubInterfaceAddressInfo{
				DeviceName:                             &deviceName,
				DeviceInterfaceName:                    utils.StringPtr(itfceInfo.GetItfceName()),
				DeviceInterfaceSubInterfaceIndex:       si.Index,
				DeviceInterfaceSubInterfaceAddressData: ad,
			}))
		}
	}
	if len(itfceInfo.GetIpv6Prefixes()) > 0 {
		for _, prefix := range itfceInfo.GetIpv6Prefixes() {
			ad, err := getAdressInfo(prefix, addressAllocationStrategy)
			if err != nil {
				return err
			}

			x.CreateDeviceInterfaceSubInterfaceAddressInfo((&DeviceInterfaceSubInterfaceAddressInfo{
				DeviceName:                             &deviceName,
				DeviceInterfaceName:                    utils.StringPtr(itfceInfo.GetItfceName()),
				DeviceInterfaceSubInterfaceIndex:       si.Index,
				DeviceInterfaceSubInterfaceAddressData: ad,
			}))
		}
	}
	return nil
}

func getAdressInfo(prefix *string, addressAllocation *nddov1.AddressAllocationStrategy) (*DeviceInterfaceSubInterfaceAddressData, error) {
	ip, n, err := net.ParseCIDR(*prefix)
	if err != nil {
		return nil, err
	}
	ipMask, _ := n.Mask.Size()
	if ip.String() == n.IP.String() && ipMask != 31 {
		switch addressAllocation.GetGatewayAllocation() {
		case nddov1.GatewayAllocationLast.String():
			ipAddr, err := GetLastIP(n)
			if err != nil {
				return nil, err
			}
			return &DeviceInterfaceSubInterfaceAddressData{
				Prefix:       utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				Cidr:         utils.StringPtr(n.String()),
				PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
				Address:      utils.StringPtr(ipAddr.String()),
			}, nil
		default:
			//case nddov1.GatewayAllocationFirst:
			ipAddr, err := GetFirstIP(n)
			if err != nil {
				return nil, err
			}
			return &DeviceInterfaceSubInterfaceAddressData{
				Prefix:       utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				Cidr:         utils.StringPtr(n.String()),
				PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
				Address:      utils.StringPtr(ipAddr.String()),
			}, nil
		}
	}
	return &DeviceInterfaceSubInterfaceAddressData{
		Prefix:       prefix,
		Cidr:         utils.StringPtr(n.String()),
		PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
		Address:      utils.StringPtr(ip.String()),
	}, nil

}

// GetLastIP returns subnet's last IP
func GetLastIP(subnet *net.IPNet) (net.IP, error) {
	size := RangeSize(subnet)
	if size <= 0 {
		return nil, fmt.Errorf("can't get range size of subnet. subnet: %q", subnet)
	}
	return GetIndexedIP(subnet, int(size-1))
}

// GetFirstIP returns subnet's last IP
func GetFirstIP(subnet *net.IPNet) (net.IP, error) {
	return GetIndexedIP(subnet, 1)
}

// RangeSize returns the size of a range in valid addresses.
func RangeSize(subnet *net.IPNet) int64 {
	ones, bits := subnet.Mask.Size()
	if bits == 32 && (bits-ones) >= 31 || bits == 128 && (bits-ones) >= 127 {
		return 0
	}
	// For IPv6, the max size will be limited to 65536
	// This is due to the allocator keeping track of all the
	// allocated IP's in a bitmap. This will keep the size of
	// the bitmap to 64k.
	if bits == 128 && (bits-ones) >= 16 {
		return int64(1) << uint(16)
	}
	return int64(1) << uint(bits-ones)
}

// GetIndexedIP returns a net.IP that is subnet.IP + index in the contiguous IP space.
func GetIndexedIP(subnet *net.IPNet, index int) (net.IP, error) {
	ip := addIPOffset(bigForIP(subnet.IP), index)
	if !subnet.Contains(ip) {
		return nil, fmt.Errorf("can't generate IP with index %d from subnet. subnet too small. subnet: %q", index, subnet)
	}
	return ip, nil
}

// addIPOffset adds the provided integer offset to a base big.Int representing a
// net.IP
func addIPOffset(base *big.Int, offset int) net.IP {
	return net.IP(big.NewInt(0).Add(base, big.NewInt(int64(offset))).Bytes())
}

// bigForIP creates a big.Int based on the provided net.IP
func bigForIP(ip net.IP) *big.Int {
	b := ip.To4()
	if b == nil {
		b = ip.To16()
	}
	return big.NewInt(0).SetBytes(b)
}
