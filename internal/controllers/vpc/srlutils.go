/*
Copyright 2021 NDD.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vpc

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"

	"github.com/openconfig/ygot/ygot"
	networkv1alpha1 "github.com/yndd/ndda-network/apis/network/v1alpha1"
	"github.com/yndd/ndda-network/pkg/ndda/itfceinfo"
	"github.com/yndd/ndda-network/pkg/ndda/niinfo"
	nddov1 "github.com/yndd/nddo-runtime/apis/common/v1"
	"github.com/yndd/nddo-runtime/pkg/resource"
	intentsrl3v1alpha1 "github.com/yndd/nddp-srl3/pkg/intent/srl3/v1alpha1"
	"github.com/yndd/nddp-srl3/pkg/ygotsrl"
)

func (r *application) PopulateSchema(ctx context.Context, mg resource.Managed, deviceName string, itfceInfo itfceinfo.ItfceInfo, niInfo *niinfo.NiInfo, addressAllocationStrategy *nddov1.AddressAllocationStrategy) error {
	crName := getCrName(mg)
	s := r.intents[crName]

	//d := s.NewDevice(r.client, deviceName).Get()

	s.AddChild(deviceName, intentsrl3v1alpha1.InitSrl(r.client, s, deviceName))
	srld := s.GetChildData(deviceName)
	d, ok := srld.(*ygotsrl.Device)
	if !ok {
		return errors.New("expected ygot struct")
	}

	niName := niInfo.GetNiName()

	ni := d.GetOrCreateNetworkInstance(niName)
	if niInfo.GetNiKind() == networkv1alpha1.E_NetworkInstanceKind_BRIDGED {
		ni.Type = ygotsrl.SrlNokiaNetworkInstance_NiType_mac_vrf
		ni.BridgeTable = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_BridgeTable{
			MacLearning: &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_BridgeTable_MacLearning{
				AdminState: ygotsrl.SrlNokiaCommon_AdminState_enable,
			},
		}
	} else {
		ni.Type = ygotsrl.SrlNokiaNetworkInstance_NiType_ip_vrf
		ni.IpForwarding = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_IpForwarding{
			ReceiveIpv4Check: ygot.Bool(true),
			ReceiveIpv6Check: ygot.Bool(true),
		}
	}
	bgpEvpn := ni.GetOrCreateProtocols().GetOrCreateBgpEvpn()
	bgpEvpnBgpInstance := bgpEvpn.GetOrCreateBgpInstance(1)
	bgpEvpnBgpInstance.AdminState = ygotsrl.SrlNokiaCommon_AdminState_enable
	bgpEvpnBgpInstance.Evi = ygot.Uint32(*niInfo.Index)
	bgpEvpnBgpInstance.Ecmp = ygot.Uint8(2)
	bgpEvpnBgpInstance.VxlanInterface = ygot.String(strings.Join([]string{"vxlan0", strconv.Itoa(int(*niInfo.Index))}, "."))

	bgpVpn := ni.GetOrCreateProtocols().GetOrCreateBgpVpn()
	bgpVpnBgpInstance := bgpVpn.GetOrCreateBgpInstance(1)
	bgpVpnBgpInstance.RouteTarget = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_BgpVpn_BgpInstance_RouteTarget{
		ImportRt: ygot.String(strings.Join([]string{"target", "65555", strconv.Itoa(int(*niInfo.Index))}, ":")),
		ExportRt: ygot.String(strings.Join([]string{"target", "65555", strconv.Itoa(int(*niInfo.Index))}, ":")),
	}

	itfceName := itfceInfo.GetItfceName()
	if strings.Contains(itfceName, "int-") {
		itfceName = strings.ReplaceAll(itfceName, "int", "ethernet")
		split := strings.Split(itfceName, "/")
		if len(split) > 2 {
			itfceName = "ethernet-" + split[len(split)-2] + "/" + split[len(split)-1]
		}
	}
	if strings.Contains(itfceName, "lag") {
		itfceName = strings.ReplaceAll(itfceName, "-", "")
	}

	// reference interface

	var ipv4 *ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4
	if len(itfceInfo.GetIpv4Prefixes()) > 0 {
		ipv4Addresses, err := getIPv4Info(itfceInfo.GetIpv4Prefixes(), addressAllocationStrategy)
		if err != nil {
			return err
		}
		ipv4 = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4{
			Address: ipv4Addresses,
		}
	}
	var ipv6 *ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6
	if len(itfceInfo.GetIpv6Prefixes()) > 0 {
		ipv6Addresses, err := getIPv6Info(itfceInfo.GetIpv6Prefixes(), addressAllocationStrategy)
		if err != nil {
			return err
		}
		ipv6 = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6{
			Address: ipv6Addresses,
		}
	}

	//var niItfceSubItfceName string
	switch itfceInfo.GetItfceKind() {
	case networkv1alpha1.E_InterfaceKind_INTERFACE:
		i := d.GetOrCreateInterface(itfceName)
		strIndex := strconv.Itoa(int(itfceInfo.GetOuterVlanId()))
		//index := itfceInfo.GetOuterVlanId()
		niItfceSubItfceName := strings.Join([]string{itfceName, strIndex}, ".")
		/*
			si := i.NewInterfaceSubinterface(r.client, srlschemav1alpha1.WithInterfaceSubinterfaceKey(&srlschemav1alpha1.InterfaceSubinterfaceKey{
				Index: strIndex,
			}))
		*/
		si := i.GetOrCreateSubinterface(uint32(itfceInfo.GetOuterVlanId()))
		if niInfo.GetNiKind() == networkv1alpha1.E_NetworkInstanceKind_BRIDGED {
			si.Type = ygotsrl.SrlNokiaInterfaces_SiType_bridged
			si.Vlan = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan{
				Encap: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap{
					SingleTagged: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap_SingleTagged{
						VlanId: ygotsrl.UnionUint16(itfceInfo.GetOuterVlanId()),
					},
				},
			}
		} else {
			si.Type = ygotsrl.SrlNokiaInterfaces_SiType_routed
			si.Vlan = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan{
				Encap: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap{
					SingleTagged: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap_SingleTagged{
						VlanId: ygotsrl.UnionUint16(itfceInfo.GetOuterVlanId()),
					},
				},
			}
			si.Ipv4 = ipv4
			si.Ipv6 = ipv6
		}
		// add subinterface to network instance
		ni.GetOrCreateInterface(niItfceSubItfceName)

	case networkv1alpha1.E_InterfaceKind_IRB:
		i := d.GetOrCreateInterface(itfceName)
		strIndex := strconv.Itoa(int(*niInfo.Index))
		niItfceSubItfceName := strings.Join([]string{itfceName, strIndex}, ".")

		si := i.GetOrCreateSubinterface(*niInfo.Index)

		if niInfo.GetNiKind() == networkv1alpha1.E_NetworkInstanceKind_BRIDGED {
			//si.Type = ygotsrl.SrlNokiaInterfaces_SiType_bridged
		} else {
			fmt.Printf("ni kind: %s\n", niInfo.GetNiKind())
			for _, a := range ipv4.Address {
				a.AnycastGw = ygot.Bool(true)
			}
			ipv4.GetOrCreateArp().LearnUnsolicited = ygot.Bool(true)
			ipv4.Arp.GetOrCreateHostRoute().GetOrCreatePopulate(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_HostRoute_Populate_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
			ipv4.Arp.GetOrCreateEvpn().GetOrCreateAdvertise(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))

			ipv6.GetOrCreateNeighborDiscovery().LearnUnsolicited = ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6_NeighborDiscovery_LearnUnsolicited_global
			ipv6.NeighborDiscovery.GetOrCreateHostRoute().GetOrCreatePopulate(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_HostRoute_Populate_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
			ipv6.NeighborDiscovery.GetOrCreateEvpn().GetOrCreateAdvertise(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))

			//si.Type = ygotsrl.SrlNokiaInterfaces_SiType_routed
			si.Ipv4 = ipv4
			si.Ipv6 = ipv6
			si.GetOrCreateAnycastGw().VirtualRouterId = ygot.Uint8(1)
		}

		// add subinterface to network instance
		ni.GetOrCreateInterface(niItfceSubItfceName)
	case networkv1alpha1.E_InterfaceKind_VXLAN:
		strIndex := strconv.Itoa(int(*niInfo.Index))
		niItfceVxlanItfceName := strings.Join([]string{itfceName, strIndex}, ".")

		fmt.Printf("ni vxlanItfce: %s\n", niItfceVxlanItfceName)
		ti := d.GetOrCreateTunnelInterface("vxlan0")
		sti := ti.GetOrCreateVxlanInterface(*niInfo.Index)

		if niInfo.GetNiKind() == networkv1alpha1.E_NetworkInstanceKind_BRIDGED {
			sti.Type = ygotsrl.SrlNokiaInterfaces_SiType_bridged
			sti.Ingress = &ygotsrl.SrlNokiaTunnelInterfaces_TunnelInterface_VxlanInterface_Ingress{
				Vni: ygot.Uint32(*niInfo.Index),
			}
			sti.Egress = &ygotsrl.SrlNokiaTunnelInterfaces_TunnelInterface_VxlanInterface_Egress{
				SourceIp: ygotsrl.SrlNokiaTunnelInterfaces_TunnelInterface_VxlanInterface_Egress_SourceIp_use_system_ipv4_address,
			}
		} else {
			sti.Type = ygotsrl.SrlNokiaInterfaces_SiType_routed
			sti.Ingress = &ygotsrl.SrlNokiaTunnelInterfaces_TunnelInterface_VxlanInterface_Ingress{
				Vni: ygot.Uint32(*niInfo.Index),
			}
			sti.Egress = &ygotsrl.SrlNokiaTunnelInterfaces_TunnelInterface_VxlanInterface_Egress{
				SourceIp: ygotsrl.SrlNokiaTunnelInterfaces_TunnelInterface_VxlanInterface_Egress_SourceIp_use_system_ipv4_address,
			}
		}
		ni.GetOrCreateVxlanInterface(niItfceVxlanItfceName)
	}

	return nil
}

func getIPv4Info(prefixes []*string, addressAllocation *nddov1.AddressAllocationStrategy) (map[string]*ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Address, error) {
	a := make(map[string]*ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Address)
	for _, prefix := range prefixes {
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
				a[strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")] = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Address{
					IpPrefix: ygot.String(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				}
			default:
				//case nddov1.GatewayAllocationFirst:
				ipAddr, err := GetFirstIP(n)
				if err != nil {
					return nil, err
				}
				a[strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")] = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Address{
					IpPrefix: ygot.String(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				}
			}
		} else {
			a[*prefix] = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Address{
				IpPrefix: ygot.String(*prefix),
			}
		}
	}
	return a, nil
}

func getIPv6Info(prefixes []*string, addressAllocation *nddov1.AddressAllocationStrategy) (map[string]*ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6_Address, error) {
	a := make(map[string]*ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6_Address)
	for _, prefix := range prefixes {
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
				a[strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")] = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6_Address{
					IpPrefix: ygot.String(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				}
			default:
				//case nddov1.GatewayAllocationFirst:
				ipAddr, err := GetFirstIP(n)
				if err != nil {
					return nil, err
				}
				a[strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")] = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6_Address{
					IpPrefix: ygot.String(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				}
			}
		} else {
			a[*prefix] = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6_Address{
				IpPrefix: ygot.String(*prefix),
			}
		}
	}
	return a, nil
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
