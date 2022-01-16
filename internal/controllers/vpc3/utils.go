package vpc3

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"

	"github.com/yndd/ndd-runtime/pkg/utils"
	networkv1alpha1 "github.com/yndd/ndda-network/apis/network/v1alpha1"
	"github.com/yndd/ndda-network/pkg/ndda/itfceinfo"
	"github.com/yndd/ndda-network/pkg/ndda/niinfo"
	networkschemav1alpha1 "github.com/yndd/ndda-network/pkg/networkschema/v1alpha1"
	nddov1 "github.com/yndd/nddo-runtime/apis/common/v1"
	"github.com/yndd/nddo-runtime/pkg/resource"
)

func (r *application) PopulateSchema(ctx context.Context, mg resource.Managed, deviceName string, itfceInfo itfceinfo.ItfceInfo, niInfo *niinfo.NiInfo, addressAllocationStrategy *nddov1.AddressAllocationStrategy) error {
	crName := getCrName(mg)
	s := r.networkHandler.InitSchema(crName)

	d := s.NewDevice(r.client, deviceName)

	i := d.NewInterface(r.client, networkschemav1alpha1.WithInterfaceKey(&networkschemav1alpha1.InterfaceKey{
		Name: itfceInfo.GetItfceName(),
	}))

	ipv4 := make([]*networkv1alpha1.InterfaceSubinterfaceIpv4, 0)
	if len(itfceInfo.GetIpv4Prefixes()) > 0 {
		for _, prefix := range itfceInfo.GetIpv4Prefixes() {
			ad, err := getIPv4Info(prefix, addressAllocationStrategy)
			if err != nil {
				return err
			}
			ipv4 = append(ipv4, ad)
		}
	}
	ipv6 := make([]*networkv1alpha1.InterfaceSubinterfaceIpv6, 0)
	if len(itfceInfo.GetIpv6Prefixes()) > 0 {
		for _, prefix := range itfceInfo.GetIpv6Prefixes() {
			ad, err := getIPv6Info(prefix, addressAllocationStrategy)
			if err != nil {
				return err
			}
			ipv6 = append(ipv6, ad)
		}
	}
	var niItfceSubItfceName string
	switch itfceInfo.GetItfceKind() {
	case networkv1alpha1.E_InterfaceKind_INTERFACE:
		index := strconv.Itoa(int(itfceInfo.GetOuterVlanId()))
		niItfceSubItfceName = strings.Join([]string{itfceInfo.GetItfceName(), index}, ".")
		si := i.NewInterfaceSubinterface(r.client, networkschemav1alpha1.WithInterfaceSubinterfaceKey(&networkschemav1alpha1.InterfaceSubinterfaceKey{
			Index: index,
		}))
		si.Update(&networkv1alpha1.InterfaceSubinterface{
			Index: utils.StringPtr(index),
			Config: &networkv1alpha1.InterfaceSubinterfaceConfig{
				Index:       utils.Uint32Ptr(uint32(itfceInfo.GetOuterVlanId())),
				Kind:        networkv1alpha1.E_InterfaceSubinterfaceKind(niInfo.Kind), // should come from itfceInfo.GetItfceKind()
				OuterVlanId: utils.Uint16Ptr(uint16(itfceInfo.GetOuterVlanId())),      // user defined, what to do when user is not defining them?
				InnerVlanId: utils.Uint16Ptr(uint16(itfceInfo.GetInnerVlanId())),      // user defined, what to do when user is not defining them?

			},
			Ipv4: ipv4,
			Ipv6: ipv6,
		})
	case networkv1alpha1.E_InterfaceKind_IRB, networkv1alpha1.E_InterfaceKind_VXLAN:
		index := strconv.Itoa(int(*niInfo.Index))
		niItfceSubItfceName = strings.Join([]string{itfceInfo.GetItfceName(), index}, ".")
		si := i.NewInterfaceSubinterface(r.client, networkschemav1alpha1.WithInterfaceSubinterfaceKey(&networkschemav1alpha1.InterfaceSubinterfaceKey{
			Index: index,
		}))
		si.Update(&networkv1alpha1.InterfaceSubinterface{
			Index: utils.StringPtr(index),
			Config: &networkv1alpha1.InterfaceSubinterfaceConfig{
				Index:       niInfo.Index,
				Kind:        networkv1alpha1.E_InterfaceSubinterfaceKind(niInfo.Kind), // should come from itfceInfo.GetItfceKind()
				OuterVlanId: utils.Uint16Ptr(uint16(itfceInfo.GetOuterVlanId())),      // user defined, what to do when user is not defining them?
				InnerVlanId: utils.Uint16Ptr(uint16(itfceInfo.GetInnerVlanId())),      // user defined, what to do when user is not defining them?

			},
			Ipv4: ipv4,
			Ipv6: ipv6,
		})
	}
	niName := strings.Join([]string{*niInfo.Name, string(niInfo.Kind)}, "-")

	ni := d.NewNetworkInstance(r.client, networkschemav1alpha1.WithNetworkInstanceKey(&networkschemav1alpha1.NetworkInstanceKey{
		Name: niName,
	}))

	ni.Update(&networkv1alpha1.NetworkInstance{
		Name: utils.StringPtr(niName),
		Config: &networkv1alpha1.NetworkInstanceConfig{
			Name: utils.StringPtr(niName),
			Kind: niInfo.Kind,
			Interface: []*networkv1alpha1.NetworkInstanceConfigInterface{
				{Name: utils.StringPtr(niItfceSubItfceName)},
			},
			Index: utils.Uint32Ptr(*niInfo.Index), // to be allocated
		},
	})
	return nil
}

func getIPv4Info(prefix *string, addressAllocation *nddov1.AddressAllocationStrategy) (*networkv1alpha1.InterfaceSubinterfaceIpv4, error) {
	ip, n, err := net.ParseCIDR(*prefix)
	if err != nil {
		return nil, err
	}
	ipMask, _ := n.Mask.Size()
	if ip.String() == n.IP.String() && ipMask != 31 {
		// none /31 interface -> allocate gw IP
		switch addressAllocation.GetGatewayAllocation() {
		case nddov1.GatewayAllocationLast.String():
			ipAddr, err := GetLastIP(n)
			if err != nil {
				return nil, err
			}
			return &networkv1alpha1.InterfaceSubinterfaceIpv4{
				IpPrefix: utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				Config: &networkv1alpha1.InterfaceSubinterfaceIpv4Config{
					IpPrefix:     utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
					IpCidr:       utils.StringPtr(n.String()),
					PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
					IpAddress:    utils.StringPtr(ipAddr.String()),
				},
			}, nil
		default:
			//case nddov1.GatewayAllocationFirst:
			ipAddr, err := GetFirstIP(n)
			if err != nil {
				return nil, err
			}
			return &networkv1alpha1.InterfaceSubinterfaceIpv4{
				IpPrefix: utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				Config: &networkv1alpha1.InterfaceSubinterfaceIpv4Config{
					IpPrefix:     utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
					IpCidr:       utils.StringPtr(n.String()),
					PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
					IpAddress:    utils.StringPtr(ipAddr.String()),
				},
			}, nil
		}
	}
	return &networkv1alpha1.InterfaceSubinterfaceIpv4{
		IpPrefix: prefix,
		Config: &networkv1alpha1.InterfaceSubinterfaceIpv4Config{
			IpPrefix:     prefix,
			IpCidr:       utils.StringPtr(n.String()),
			PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
			IpAddress:    utils.StringPtr(ip.String()),
		},
	}, nil
}

func getIPv6Info(prefix *string, addressAllocation *nddov1.AddressAllocationStrategy) (*networkv1alpha1.InterfaceSubinterfaceIpv6, error) {
	ip, n, err := net.ParseCIDR(*prefix)
	if err != nil {
		return nil, err
	}
	ipMask, _ := n.Mask.Size()
	if ip.String() == n.IP.String() && ipMask != 127 {
		// none /127 interface -> allocate gw IP
		switch addressAllocation.GetGatewayAllocation() {
		case nddov1.GatewayAllocationLast.String():
			ipAddr, err := GetLastIP(n)
			if err != nil {
				return nil, err
			}
			return &networkv1alpha1.InterfaceSubinterfaceIpv6{
				IpPrefix: utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				Config: &networkv1alpha1.InterfaceSubinterfaceIpv6Config{
					IpPrefix:     utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
					IpCidr:       utils.StringPtr(n.String()),
					PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
					IpAddress:    utils.StringPtr(ipAddr.String()),
				},
			}, nil
		default:
			//case nddov1.GatewayAllocationFirst:
			ipAddr, err := GetFirstIP(n)
			if err != nil {
				return nil, err
			}
			return &networkv1alpha1.InterfaceSubinterfaceIpv6{
				IpPrefix: utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				Config: &networkv1alpha1.InterfaceSubinterfaceIpv6Config{
					IpPrefix:     utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
					IpCidr:       utils.StringPtr(n.String()),
					PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
					IpAddress:    utils.StringPtr(ipAddr.String()),
				},
			}, nil
		}
	}
	return &networkv1alpha1.InterfaceSubinterfaceIpv6{
		IpPrefix: prefix,
		Config: &networkv1alpha1.InterfaceSubinterfaceIpv6Config{
			IpPrefix:     prefix,
			IpCidr:       utils.StringPtr(n.String()),
			PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
			IpAddress:    utils.StringPtr(ip.String()),
		},
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

func hash(key string) uint32 {
	sum := 0
	for _, v := range key {
		sum += int(v)
	}
	return uint32(sum) % 10000
}
