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

/*
func (r *application) createGlobalNetworkInstance(ctx context.Context, cr vpcv1alpha1.Vp, nip niselector.ItfceInfo) (infra.Ni, error) {

	nis := r.handler.GetInfraNis(getCrName(cr))

	niName := nip.GetNiName()
	r.log.Debug("createGlobalNetworkInstance", "niName", niName)
	ipamClient := nip.GetGlobalParamaters().GetResourceClient(registry.RegisterKindIpam.String())
	aspoolClient := nip.GetGlobalParamaters().GetResourceClient(registry.RegisterKindAs.String())
	niClient := nip.GetGlobalParamaters().GetResourceClient(registry.RegisterKindNi.String())

	if _, ok := nis[niName]; !ok {
		nis[niName] = infra.NewNi(nil, niName,
			infra.WithNiK8sClient(r.client),
			infra.WithNiIpamClient(ipamClient),
			infra.WithNiAsPoolClient(aspoolClient),
			infra.WithNiNiClient(niClient),
			infra.WithNiLogger(r.log),
		)
	}
	return nis[niName], nil
}

func (r *application) getGlobalNetworkInstance(crName, niName string) (infra.Ni, error) {
	nis := r.handler.GetInfraNis(crName)

	if _, ok := nis[niName]; !ok {
		return nil, fmt.Errorf("niName %s not found", niName)
	}
	return nis[niName], nil
}

func (r *application) createNetworkInstanceSubInterfaces(ctx context.Context, cr vpcv1alpha1.Vp, nip niselector.ItfceInfo) (infra.SubInterface, error) {
	nodes := r.handler.GetInfraNodes(getCrName(cr))

	ipamClient := nip.GetGlobalParamaters().GetResourceClient("ipam")
	aspoolClient := nip.GetGlobalParamaters().GetResourceClient("aspool")
	addressAllocationStrategy := nip.GetGlobalParamaters().GetAddressAllocationStrategy()
	nodeName := nip.GetNodeName()
	itfceName := nip.GetItfceName()
	itfceIndex := nip.GetItfceIndex()
	vlanId := nip.GetVlanId()
	niName := nip.GetNiName()
	niKind := nip.GetNiKind()
	epgName := nip.GetEpgName()
	ipv4Prefixes := nip.GetIpv4Prefixes()
	ipv6Prefixes := nip.GetIpv6Prefixes()

	if _, ok := nodes[nodeName]; !ok {
		nodes[nodeName] = infra.NewNode(nodeName,
			infra.WithNodeK8sClient(r.client),
			infra.WithNodeIpamClient(ipamClient),
			infra.WithNodeAsPoolClient(aspoolClient),
			infra.WithNodeLogger(r.log))
	}
	n := nodes[nodeName]
	if _, ok := n.GetInterfaces()[itfceName]; !ok {
		n.GetInterfaces()[itfceName] = infra.NewInterface(n, itfceName,
			infra.WithInterfaceK8sClient(r.client),
			infra.WithInterfaceIpamClient(ipamClient),
			infra.WithInterfaceAsPoolClient(aspoolClient),
			infra.WithInterfaceLogger(r.log))
	}
	itfce := n.GetInterfaces()[itfceName]

	if _, ok := itfce.GetSubInterfaces()[itfceIndex]; !ok {
		itfce.GetSubInterfaces()[itfceIndex] = infra.NewSubInterface(itfce, itfceIndex,
			infra.WithSubInterfaceK8sClient(r.client),
			infra.WithSubInterfaceIpamClient(ipamClient),
			infra.WithSubInterfaceAsPoolClient(aspoolClient),
			infra.WithSubInterfaceLogger(r.log))
	}
	si := itfce.GetSubInterfaces()[itfceIndex]
	si.SetKind(infra.SubInterfaceKind(niKind))
	si.SetTaggingKind(infra.TaggingKindSingleTagged)
	si.SetOuterTag(vlanId)
	si.SetEpgName(epgName)
	si.SetItfceSelectorKind(nip.GetItfceSelectorKind())
	si.SetItfceSelectorTags(nip.GetItfceSelectorTags())
	si.SetBridgeDomainName(nip.GetBridgeDomainName())

	if len(ipv4Prefixes) > 0 {
		for _, prefix := range ipv4Prefixes {

			si.GetAddressesInfo("ipv4")[*prefix] = infra.NewAddressInfo(si, *prefix,
				infra.WithAddressK8sClient(r.client),
				infra.WithAddressIpamClient(ipamClient),
				infra.WithAddressAsPoolClient(aspoolClient),
				infra.WithAddressLogger(r.log))

			ip, n, err := net.ParseCIDR(*prefix)
			if err != nil {
				return nil, err
			}
			ipMask, _ := n.Mask.Size()
			if ip.String() == n.IP.String() && ipMask != 31 {
				// this is a prefix without a specific address -> we need to allocate an IP from the subnet
				si.GetAddressesInfo("ipv4")[*prefix].SetCidr(n.String())
				si.GetAddressesInfo("ipv4")[*prefix].SetPrefixLength(uint32(ipMask))

				switch *addressAllocationStrategy.GatewayAllocation {
				case nddov1.GatewayAllocationFirst:
					ipAddr, err := GetFirstIP(n)
					if err != nil {
						return nil, err
					}
					si.GetAddressesInfo("ipv4")[*prefix].SetAddress(ipAddr.String())
					si.GetAddressesInfo("ipv4")[*prefix].SetPrefix(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/"))
				case nddov1.GatewayAllocationLast:
					ipAddr, err := GetLastIP(n)
					if err != nil {
						return nil, err
					}
					si.GetAddressesInfo("ipv4")[*prefix].SetAddress(ipAddr.String())
					si.GetAddressesInfo("ipv4")[*prefix].SetPrefix(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/"))
				}
			} else {
				// this is a prefix with a specific address
				si.GetAddressesInfo("ipv4")[*prefix].SetCidr(n.String()) // this should provide the cidr (0 address)
				si.GetAddressesInfo("ipv4")[*prefix].SetPrefix(*prefix)  // this is the original prefix
				si.GetAddressesInfo("ipv4")[*prefix].SetAddress(ip.String())
				si.GetAddressesInfo("ipv4")[*prefix].SetPrefixLength(uint32(ipMask))
			}
			fmt.Printf("prefix: %s, newprefix: %s, prefixlength: %d, address: %s, cidr: %t \n", *prefix, n.String(), ipMask, ip.String(), ip.String() == n.IP.String())
		}
	}
	if len(ipv6Prefixes) > 0 {
		for _, prefix := range ipv6Prefixes {

			si.GetAddressesInfo("ipv6")[*prefix] = infra.NewAddressInfo(si, *prefix,
				infra.WithAddressK8sClient(r.client),
				infra.WithAddressIpamClient(ipamClient),
				infra.WithAddressAsPoolClient(aspoolClient),
				infra.WithAddressLogger(r.log))

			ip, n, err := net.ParseCIDR(*prefix)
			if err != nil {
				return nil, err
			}
			ipMask, _ := n.Mask.Size()
			if ip.String() == n.IP.String() && ipMask != 127 {
				// this is a prefix without a specific address
				si.GetAddressesInfo("ipv6")[*prefix].SetCidr(n.String())
				si.GetAddressesInfo("ipv6")[*prefix].SetPrefixLength(uint32(ipMask))
				switch *addressAllocationStrategy.GatewayAllocation {
				case nddov1.GatewayAllocationFirst:
					ipAddr, err := GetFirstIP(n)
					if err != nil {
						return nil, err
					}
					si.GetAddressesInfo("ipv6")[*prefix].SetAddress(ipAddr.String())
					si.GetAddressesInfo("ipv6")[*prefix].SetPrefix(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/"))
				case nddov1.GatewayAllocationLast:
					ipAddr, err := GetLastIP(n)
					if err != nil {
						return nil, err
					}
					si.GetAddressesInfo("ipv6")[*prefix].SetAddress(ipAddr.String())
					si.GetAddressesInfo("ipv6")[*prefix].SetPrefix(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/"))
				}
				fmt.Printf("address: %s, prefix: %s, cidr: %s, pl: %d \n",
					si.GetAddressesInfo("ipv6")[*prefix].GetAddress(),
					si.GetAddressesInfo("ipv6")[*prefix].GetPrefix(),
					si.GetAddressesInfo("ipv6")[*prefix].GetCidr(),
					si.GetAddressesInfo("ipv6")[*prefix].GetPrefixLength())
			} else {
				// this is a prefix with a specific address
				si.GetAddressesInfo("ipv6")[*prefix].SetCidr(n.String()) // this should provide the cidr (0 address)
				si.GetAddressesInfo("ipv6")[*prefix].SetPrefix(*prefix)
				si.GetAddressesInfo("ipv6")[*prefix].SetAddress(ip.String())
				si.GetAddressesInfo("ipv6")[*prefix].SetPrefixLength(uint32(ipMask))
			}
			fmt.Printf("prefix: %s, newprefix: %s, prefixlength: %d, address: %s, cidr: %t \n", *prefix, n.String(), ipMask, ip.String(), ip.String() == n.IP.String())
		}
	}

	if _, ok := n.GetNis()[niName]; !ok {
		n.GetNis()[niName] = infra.NewNi(n, niName,
			infra.WithNiK8sClient(r.client),
			infra.WithNiIpamClient(ipamClient),
			infra.WithNiAsPoolClient(aspoolClient),
			infra.WithNiLogger(r.log),
		)
	}

	ni := n.GetNis()[niName]
	ni.AddSubInterface(si, itfce)
	si.SetNi(ni)
	//ni.GetSubInterfaces()[nip.nisiName] = si
	ni.SetKind(infra.NiKind(niKind))

	return si, nil

}
*/
/*
func getSelectedNodeItfces(epgSelectors []*nddov1.EpgInfo, nodeItfceSelectors map[string]*nddov1.ItfceInfo, nddaItfceList networkv1alpha1.IfList) map[string][]niselector.ItfceInfo {
	s := niselector.NewNodeItfceSelection()
	s.GetNodeItfcesByEpgSelector(epgSelectors, nddaItfceList)
	s.GetNodeItfcesByNodeItfceSelector(nodeItfceSelectors, nddaItfceList)
	return s.GetSelectedNodeItfces()
}
*/

/*
func getIrbNodeItfces(bd *vpcv1alpha1.VpcVpcRoutingTablesBridgeDomains, activeNiNodeAndLinks []niselector.ItfceInfo, nddaItfceList networkv1alpha1.IfList) map[string][]niselector.ItfceInfo {
	s := niselector.NewNodeItfceSelection()
	s.GetIrbNodeItfces(strings.Join([]string{bd.GetName(), infra.NiKindBridged.String()}, "-"), activeNiNodeAndLinks, nddaItfceList, bd.GetIPv4Prefixes(), bd.GetIPv6Prefixes())
	return s.GetSelectedNodeItfces()
}



func getVxlanNodeItfces(niName string, activeNiNodeAndLinks []niselector.ItfceInfo, nddaItfceList networkv1alpha1.IfList) map[string][]niselector.ItfceInfo {
	s := niselector.NewNodeItfceSelection()
	s.GetVxlanNodeItfces(strings.Join([]string{niName, infra.NiKindBridged.String()}, "-"), activeNiNodeAndLinks, nddaItfceList)
	return s.GetSelectedNodeItfces()
}
*/

/*
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
*/
