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

package vpc2

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/yndd/ndd-runtime/pkg/event"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/ndd-runtime/pkg/utils"
	networkv1alpha1 "github.com/yndd/ndda-network/apis/network/v1alpha1"
	nddov1 "github.com/yndd/nddo-runtime/apis/common/v1"
	"github.com/yndd/nddo-runtime/pkg/niselector"
	"github.com/yndd/nddo-runtime/pkg/reconciler/managed"
	"github.com/yndd/nddo-runtime/pkg/resource"
	vpcv1alpha1 "github.com/yndd/nddo-vpc/apis/vpc/v1alpha1"
	"github.com/yndd/nddo-vpc/internal/schema"
	"github.com/yndd/nddo-vpc/internal/schemahandler"
	"github.com/yndd/nddo-vpc/internal/shared"
	"github.com/yndd/nddr-org-registry/pkg/registry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// timers
	reconcileTimeout = 1 * time.Minute
	shortWait        = 5 * time.Second
	veryShortWait    = 1 * time.Second
	// errors
	errUnexpectedResource = "unexpected infrastructure object"
	errGetK8sResource     = "cannot get infrastructure resource"
)

// Setup adds a controller that reconciles infra.
func Setup(mgr ctrl.Manager, o controller.Options, nddcopts *shared.NddControllerOptions) error {
	name := "nddo/" + strings.ToLower(vpcv1alpha1.VpcGroupKind)
	vpcfn := func() vpcv1alpha1.Vp { return &vpcv1alpha1.Vpc{} }
	vpclfn := func() vpcv1alpha1.VpList { return &vpcv1alpha1.VpcList{} }
	nddaiflfn := func() networkv1alpha1.IfList { return &networkv1alpha1.InterfaceList{} }
	nddasilfn := func() networkv1alpha1.SiList { return &networkv1alpha1.SubInterfaceList{} }
	nddanilfn := func() networkv1alpha1.NiList { return &networkv1alpha1.NetworkInstanceList{} }

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(vpcv1alpha1.VpcGroupVersionKind),
		managed.WithLogger(nddcopts.Logger.WithValues("controller", name)),
		managed.WithApplication(&application{
			client: resource.ClientApplicator{
				Client:     mgr.GetClient(),
				Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
			},
			log:                     nddcopts.Logger.WithValues("applogic", name),
			newVpc:                  vpcfn,
			newVpcList:              vpclfn,
			newNddaItfceList:        nddaiflfn,
			newNddaSubInterfaceList: nddasilfn,
			newNddaNiList:           nddanilfn,
			handler:                 nddcopts.SchemaHandler,
			registry:                nddcopts.Registry,
		}),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	nddaItfceHandler := &EnqueueRequestForAllNddaInterfaces{
		client:     mgr.GetClient(),
		log:        nddcopts.Logger,
		ctx:        context.Background(),
		newVpcList: vpclfn,
		handler:    nddcopts.Handler,
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o).
		For(&vpcv1alpha1.Vpc{}).
		Owns(&vpcv1alpha1.Vpc{}).
		WithEventFilter(resource.IgnoreUpdateWithoutGenerationChangePredicate()).
		Watches(&source.Kind{Type: &networkv1alpha1.Interface{}}, nddaItfceHandler).
		WithEventFilter(resource.IgnoreUpdateWithoutGenerationChangePredicate()).
		Complete(r)

}

type application struct {
	client resource.ClientApplicator
	log    logging.Logger

	newVpc                  func() vpcv1alpha1.Vp
	newVpcList              func() vpcv1alpha1.VpList
	newNddaItfceList        func() networkv1alpha1.IfList
	newNddaSubInterfaceList func() networkv1alpha1.SiList
	newNddaNiList           func() networkv1alpha1.NiList

	handler  schemahandler.Handler
	registry registry.Registry
}

func getCrName(cr vpcv1alpha1.Vp) string {
	return strings.Join([]string{cr.GetNamespace(), cr.GetName()}, ".")
}

func getCrNameCandidate(mg resource.Managed) string {
	return strings.Join([]string{mg.GetNamespace(), mg.GetName(), "candidate"}, ".")
}

func (r *application) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return errors.New(errUnexpectedResource)
	}

	if err := cr.InitializeResource(); err != nil {
		r.log.Debug("Cannot initialize", "error", err)
		return err
	}

	return nil
}

func (r *application) Update(ctx context.Context, mg resource.Managed) (map[string]string, error) {
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return nil, errors.New(errUnexpectedResource)
	}

	return nil, r.populateSchema(ctx, mg)

	return r.handleAppLogic(ctx, cr)
}

func (r *application) FinalUpdate(ctx context.Context, mg resource.Managed) {
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return
	}
	r.handler.PrintSchemaDevices(getCrName(cr))
}

func (r *application) Timeout(ctx context.Context, mg resource.Managed) time.Duration {
	_, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return reconcileTimeout
	}
	/*
		speedy := r.handler.GetSpeedy(getCrName(cr))
		if speedy <= 2 {
			r.handler.IncrementSpeedy(getCrName(cr))
			r.log.Debug("Speedy incr", "number", r.handler.GetSpeedy(getCrName(cr)))
			switch speedy {
			case 0:
				return veryShortWait
			case 1, 2:
				return shortWait
			}
		}
	*/
	return reconcileTimeout
}

func (r *application) Delete(ctx context.Context, mg resource.Managed) (bool, error) {
	return true, nil
}

func (r *application) FinalDelete(ctx context.Context, mg resource.Managed) {
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return
	}
	r.handler.DestroySchema(getCrName(cr))
}
func (r *application) handleAppLogic(ctx context.Context, cr vpcv1alpha1.Vp) (map[string]string, error) {

	// r.allocateResources(ctx, cr)

	// r.validateDelta(ctx, cr)

	// r.applyIntent(ctx, cr)

	return nil, nil
}

func (r *application) populateSchema(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return errors.New(errUnexpectedResource)
	}

	// initialize the cr elements
	s := r.handler.NewSchema(getCrNameCandidate(mg))

	addressAllocationStrategy, err := r.registry.GetAddressAllocationStrategy(ctx, mg)
	if err != nil {
		return err
	}

	// get all ndda interfaces
	nddaItfces := r.newNddaItfceList()
	if err := r.client.List(ctx, nddaItfces); err != nil {
		return err
	}

	// get AS -> depending on if this is per org or per deployment

	for bdName := range cr.GetBridgeDomains() {
		// get AS
		// allocate RT
		// get the epg, nodeitfce selectors belonging to this bridge domain
		// this validates and transforms the user input
		epgSelectors, nodeItfceSelectors, err := cr.GetBridgeEpgAndNodeItfceSelectors(bdName)
		if err != nil {
			return err
		}
		// get the nodes and interfaces based on the epg, nodeitfce selectors
		selectedNodeItfces := getSelectedNodeItfces(epgSelectors, nodeItfceSelectors, nddaItfces)
		for nodeName, itfces := range selectedNodeItfces {
			for _, itfceInfo := range itfces {
				s.CreateDevice(&schema.Device{
					Name: &nodeName,
				})

				s.CreateDeviceInterface(&schema.DeviceInterface{
					DeviceName: &nodeName,
					DeviceInterfaceData: &schema.DeviceInterfaceData{
						Name: utils.StringPtr(itfceInfo.GetItfceName()),
					},
				})

				s.CreateDeviceInterfaceSubInterface(&schema.DeviceInterfaceSubInterface{
					DeviceName:                &nodeName,
					DeviceInterfaceName:       utils.StringPtr(itfceInfo.GetItfceName()),
					DeviceNetworkInstanceName: utils.StringPtr(strings.Join([]string{bdName, "bridged"}, "-")),
					DeviceInterfaceSubInterfaceData: &schema.DeviceInterfaceSubInterfaceData{
						Kind:     utils.StringPtr("interface"),
						OuterTag: utils.Uint32Ptr(0), // to be allocated locally -> where is this coming from?
						InnerTag: utils.Uint32Ptr(0), // to be allocated locally -> where is this coming from?

					},
				})

				s.CreateDeviceNetworkInstance(&schema.DeviceNetworkInstance{
					DeviceName: &nodeName,
					DeviceNetworkInstanceData: &schema.DeviceNetworkInstanceData{
						Name:  utils.StringPtr(strings.Join([]string{bdName, "bridged"}, "-")),
						Kind:  utils.StringPtr("bridged"),
						Index: utils.Uint32Ptr(0), // to be allocated locally
					},
				})
			}
		}

		// if the tunnel mechanism in the bridge domin is vxlan add the vxlan interface
		if cr.GetBridgeDomainTunnel(bdName) == vpcv1alpha1.TunnelVxlan.String() {
			selectedNodeItfces := getVxlanNodeItfces(bdName, s, nddaItfces)
			for nodeName, itfces := range selectedNodeItfces {
				for _, itfceInfo := range itfces {

					s.CreateDeviceInterface(&schema.DeviceInterface{
						DeviceName: &nodeName,
						DeviceInterfaceData: &schema.DeviceInterfaceData{
							Name: utils.StringPtr(itfceInfo.GetItfceName()),
						},
					})

					s.CreateDeviceInterfaceSubInterface(&schema.DeviceInterfaceSubInterface{
						DeviceName:                &nodeName,
						DeviceInterfaceName:       utils.StringPtr(itfceInfo.GetItfceName()),
						DeviceNetworkInstanceName: utils.StringPtr(strings.Join([]string{bdName, "bridged"}, "-")),
						DeviceInterfaceSubInterfaceData: &schema.DeviceInterfaceSubInterfaceData{
							Kind: utils.StringPtr("vxlan"),
						},
					})
				}
			}
		}
	}

	for rtName := range cr.GetRoutingTables() {
		// allocate RT
		// get the epg, nodeitfce selectors belonging to this bridge domain
		epgSelectors, nodeItfceSelectors, err := cr.GetRoutingTableEpgAndNodeItfceSelectors(rtName)
		if err != nil {
			return err
		}
		//log.Debug("routed epgSelectors", "epgSelectors", epgSelectors)
		//log.Debug("routed nodeItfceSelectors", "nodeItfceSelectors", nodeItfceSelectors)
		// get the nodes and interfaces based on the epg, nodeitfce selectors
		selectedNodeItfces := getSelectedNodeItfces(epgSelectors, nodeItfceSelectors, nddaItfces)
		for nodeName, itfces := range selectedNodeItfces {
			for _, itfceInfo := range itfces {
				s.CreateDevice(&schema.Device{
					Name: &nodeName,
				})

				s.CreateDeviceInterface(&schema.DeviceInterface{
					DeviceName: &nodeName,
					DeviceInterfaceData: &schema.DeviceInterfaceData{
						Name: utils.StringPtr(itfceInfo.GetItfceName()),
					},
				})

				s.CreateDeviceInterfaceSubInterface(&schema.DeviceInterfaceSubInterface{
					DeviceName:                &nodeName,
					DeviceInterfaceName:       utils.StringPtr(itfceInfo.GetItfceName()),
					DeviceNetworkInstanceName: utils.StringPtr(strings.Join([]string{rtName, "routed"}, "-")),
					DeviceInterfaceSubInterfaceData: &schema.DeviceInterfaceSubInterfaceData{
						Index:    utils.Uint32Ptr(0), // tbd should be vlanid
						Kind:     utils.StringPtr("interface"),
						OuterTag: utils.Uint32Ptr(0), // to be allocated locally -> where is this coming from?
						InnerTag: utils.Uint32Ptr(0), // to be allocated locally -> where is this coming from?
					},
				})

				s.CreateDeviceNetworkInstance(&schema.DeviceNetworkInstance{
					DeviceName: &nodeName,
					DeviceNetworkInstanceData: &schema.DeviceNetworkInstanceData{
						Name:  utils.StringPtr(strings.Join([]string{rtName, "routed"}, "-")),
						Kind:  utils.StringPtr("routed"),
						Index: utils.Uint32Ptr(0), // to be allocated locally
					},
				})

				// TBD do we need an indication for address allocation
				if len(itfceInfo.GetIpv4Prefixes()) > 0 {
					for _, prefix := range itfceInfo.GetIpv4Prefixes() {

						ad, err := getAdressInfo(prefix, addressAllocationStrategy)
						if err != nil {
							return err
						}

						s.CreateDeviceInterfaceSubInterfaceAddressInfo((&schema.DeviceInterfaceSubInterfaceAddressInfo{
							DeviceName:                             &nodeName,
							DeviceInterfaceName:                    utils.StringPtr(itfceInfo.GetItfceName()),
							DeviceInterfaceSubInterfaceIndex:       utils.Uint32Ptr(0),
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

						s.CreateDeviceInterfaceSubInterfaceAddressInfo((&schema.DeviceInterfaceSubInterfaceAddressInfo{
							DeviceName:                             &nodeName,
							DeviceInterfaceName:                    utils.StringPtr(itfceInfo.GetItfceName()),
							DeviceInterfaceSubInterfaceIndex:       utils.Uint32Ptr(0),
							DeviceInterfaceSubInterfaceAddressData: ad,
						}))

					}
				}
			}
		}
		// if the tunnel mechanism in the bridge domin is vxlan add the vxlan interface
		if cr.GetBridgeDomainTunnel(rtName) == vpcv1alpha1.TunnelVxlan.String() {
			selectedNodeItfces := getVxlanNodeItfces(rtName, s, nddaItfces)
			for nodeName, itfces := range selectedNodeItfces {
				for _, itfceInfo := range itfces {

					s.CreateDeviceInterface(&schema.DeviceInterface{
						DeviceName: &nodeName,
						DeviceInterfaceData: &schema.DeviceInterfaceData{
							Name: utils.StringPtr(itfceInfo.GetItfceName()),
						},
					})

					s.CreateDeviceInterfaceSubInterface(&schema.DeviceInterfaceSubInterface{
						DeviceName:                &nodeName,
						DeviceInterfaceName:       utils.StringPtr(itfceInfo.GetItfceName()),
						DeviceNetworkInstanceName: utils.StringPtr(strings.Join([]string{rtName, "routed"}, "-")),
						DeviceInterfaceSubInterfaceData: &schema.DeviceInterfaceSubInterfaceData{
							Kind: utils.StringPtr("vxlan"),
						},
					})
				}
			}
		}
	}
	// handle irb
	for rtName, bds := range cr.GetBridge2RoutingTable() {
		for _, bd := range bds {
			bdfound := false
			for bdName := range cr.GetBridgeDomains() {
				if bd.GetName() == bdName {
					bdfound = true
					break
				}
			}
			if bdfound {
				selectedNodeItfces := getIrbNodeItfces(bd, s, nddaItfces)
				for nodeName, itfces := range selectedNodeItfces {
					for _, itfceInfo := range itfces {
						s.CreateDeviceInterfaceSubInterface(&schema.DeviceInterfaceSubInterface{
							DeviceName:                &nodeName,
							DeviceInterfaceName:       utils.StringPtr(itfceInfo.GetItfceName()),
							DeviceNetworkInstanceName: utils.StringPtr(strings.Join([]string{rtName, "routed"}, "-")),
							DeviceInterfaceSubInterfaceData: &schema.DeviceInterfaceSubInterfaceData{
								Index: utils.Uint32Ptr(0), // tbd
								Kind:  utils.StringPtr("irb"),
							},
						})

						s.CreateDeviceInterfaceSubInterface(&schema.DeviceInterfaceSubInterface{
							DeviceName:                &nodeName,
							DeviceInterfaceName:       utils.StringPtr(itfceInfo.GetItfceName()),
							DeviceNetworkInstanceName: utils.StringPtr(strings.Join([]string{rtName, "bridged"}, "-")),
							DeviceInterfaceSubInterfaceData: &schema.DeviceInterfaceSubInterfaceData{
								Index: utils.Uint32Ptr(0), // tbd
								Kind:  utils.StringPtr("irb"),
							},
						})
					}
				}
			}
		}
	}

	return nil
}

func getSelectedNodeItfces(epgSelectors []*nddov1.EpgInfo, nodeItfceSelectors map[string]*nddov1.ItfceInfo, nddaItfceList networkv1alpha1.IfList) map[string][]niselector.ItfceInfo {
	s := niselector.NewNodeItfceSelection()
	s.GetNodeItfcesByEpgSelector(epgSelectors, nddaItfceList)
	s.GetNodeItfcesByNodeItfceSelector(nodeItfceSelectors, nddaItfceList)
	return s.GetSelectedNodeItfces()
}

func getVxlanNodeItfces(niName string, s schema.Schema, nddaItfceList networkv1alpha1.IfList) map[string][]niselector.ItfceInfo {
	sel := niselector.NewNodeItfceSelection()
	sel.GetVxlanNodeItfces(strings.Join([]string{niName, "bridged"}, "-"), s, nddaItfceList)
	return s.GetSelectedNodeItfces()
}

func getIrbNodeItfces(bd *vpcv1alpha1.VpcVpcRoutingTablesBridgeDomains, s schema.Schema, nddaItfceList networkv1alpha1.IfList) map[string][]niselector.ItfceInfo {
	sel := niselector.NewNodeItfceSelection()
	sel.GetIrbNodeItfces(strings.Join([]string{bd.GetName(), "bridged"}, "-"), s, nddaItfceList, bd.GetIPv4Prefixes(), bd.GetIPv6Prefixes())
	return s.GetSelectedNodeItfces()
}

func getAdressInfo(prefix *string, addressAllocation *nddov1.AddressAllocationStrategy) (*schema.DeviceInterfaceSubInterfaceAddressData, error) {
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
			return &schema.DeviceInterfaceSubInterfaceAddressData{
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
			return &schema.DeviceInterfaceSubInterfaceAddressData{
				Prefix:       utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				Cidr:         utils.StringPtr(n.String()),
				PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
				Address:      utils.StringPtr(ipAddr.String()),
			}, nil
		}
	}
	return &schema.DeviceInterfaceSubInterfaceAddressData{
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
