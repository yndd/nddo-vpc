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
	"strconv"
	"strings"
	"time"

	"github.com/yndd/ndd-runtime/pkg/event"
	"github.com/yndd/ndd-runtime/pkg/logging"
	networkv1alpha1 "github.com/yndd/ndda-network/apis/network/v1alpha1"
	"github.com/yndd/nddo-runtime/pkg/niselector"
	"github.com/yndd/nddo-runtime/pkg/odns"
	"github.com/yndd/nddo-runtime/pkg/reconciler/managed"
	"github.com/yndd/nddo-runtime/pkg/resource"
	vpcv1alpha1 "github.com/yndd/nddo-vpc/apis/vpc/v1alpha1"
	"github.com/yndd/nddo-vpc/internal/handler"
	"github.com/yndd/nddo-vpc/internal/infra"
	"github.com/yndd/nddo-vpc/internal/shared"
	ipamv1alpha1 "github.com/yndd/nddr-ipam-registry/apis/ipam/v1alpha1"
	"github.com/yndd/nddr-org-registry/pkg/registry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
			handler:                 nddcopts.Handler,
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

	handler  handler.Handler
	registry registry.Registry
}

func getCrName(mg resource.Managed) string {
	return strings.Join([]string{mg.GetNamespace(), mg.GetName()}, ".")
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
	

	return r.handleAppLogic(ctx, mg)
}

func (r *application) FinalUpdate(ctx context.Context, mg resource.Managed) {
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return
	}
	r.handler.PrintInfraNodes(getCrName(cr))
}

func (r *application) Timeout(ctx context.Context, mg resource.Managed) time.Duration {
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return reconcileTimeout
	}
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
	r.handler.Delete(getCrName(cr))
}

func (r *application) handleAppLogic(ctx context.Context, mg resource.Managed) (map[string]string, error) {
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return nil, errors.New(errUnexpectedResource)
	}
	log := r.log.WithValues("function", "handleAppLogic", "crname", mg.GetName())
	log.Debug("handleAppLogic")

	// initialize the cr elements
	r.handler.Init(getCrName(mg))

	// Registers and grpc server

	register, err := r.registry.GetRegister(ctx, mg)
	if err != nil {
		return nil, err
	}

	addressAllocationStrategy, err := r.registry.GetAddressAllocationStrategy(ctx, mg)
	if err != nil {
		return nil, err
	}

	//log.Debug("addressAllocationStrategy", "addressAllocationStrategy", addressAllocationStrategy)

	ipamClient, err := r.registry.GetRegistryClient(ctx, registry.RegisterKindIpam.String())
	if err != nil {
		return nil, err
	}

	aspoolClient, err := r.registry.GetRegistryClient(ctx, registry.RegisterKindAs.String())
	if err != nil {
		return nil, err
	}

	niregistryClient, err := r.registry.GetRegistryClient(ctx, registry.RegisterKindNi.String())
	if err != nil {
		return nil, err
	}

	ipamName := register[registry.RegisterKindIpam.String()]
	niRegistryName := register[registry.RegisterKindNi.String()]

	// get all ndda interfaces
	nddaItfces := r.newNddaItfceList()
	if err := r.client.List(ctx, nddaItfces); err != nil {
		return nil, err
	}

	activeNiNodeAndLinks := make([]niselector.ItfceInfo, 0)
	for bdName := range cr.GetBridgeDomains() {

		nip := niselector.NewItfceInfo(
			niselector.WithResourceClient(registry.RegisterKindIpam.String(), ipamClient),
			niselector.WithResourceClient(registry.RegisterKindAs.String(), aspoolClient),
			niselector.WithResourceClient(registry.RegisterKindNi.String(), niregistryClient),
			niselector.WithNiName(strings.Join([]string{bdName, infra.NiKindBridged.String()}, "-")),
			niselector.WithNiKind(infra.NiKindBridged.String()),
		)
		ni, err := r.createGlobalNetworkInstance(ctx, cr, nip)
		if err != nil {
			return nil, err
		}

		niOptions := &infra.NiOptions{
			RegistryName:        niRegistryName,
			NetworkInstanceName: nip.GetNiName(),
		}
		niIndex, err := ni.GrpcAllocateNiIndex(ctx, cr, niOptions)
		if err != nil {
			return nil, err
		}
		ni.SetIndex(niIndex)

		// get the epg, nodeitfce selectors belonging to this bridge domain
		// this validates and transforms the user input
		epgSelectors, nodeItfceSelectors, err := cr.GetBridgeEpgAndNodeItfceSelectors(bdName)
		if err != nil {
			return nil, err
		}
		// get the nodes and interfaces based on the epg, nodeitfce selectors
		selectedNodeItfces := getSelectedNodeItfces(epgSelectors, nodeItfceSelectors, nddaItfces)
		//fmt.Printf("bd selectedNodeItfces: %v\n", selectedNodeItfces)
		for nodeName, itfces := range selectedNodeItfces {
			log.Debug("bridge domain interfaces", "bdName", bdName, "node", nodeName)
			for _, itfceInfo := range itfces {
				log.Debug("bridge domain interface",
					"bdName", bdName, "node", nodeName,
					"itfce", itfceInfo.GetItfceName(),
					"vlanid", itfceInfo.GetVlanId())

				nip := niselector.NewItfceInfo(
					niselector.WithRegister(register),
					niselector.WithAddressAllocationStrategy(addressAllocationStrategy),
					niselector.WithResourceClient("ipam", ipamClient),
					niselector.WithResourceClient("aspool", aspoolClient),
					niselector.WithResourceClient("ni-registry", niregistryClient),
					niselector.WithEpgName(itfceInfo.GetEpgName()),
					niselector.WithNiName(strings.Join([]string{bdName, infra.NiKindBridged.String()}, "-")),
					niselector.WithNiKind(infra.NiKindBridged.String()),
					niselector.WithNodeName(nodeName),
					niselector.WithItfceName(itfceInfo.GetItfceName()),
					niselector.WithItfceIndex(strconv.Itoa(int(itfceInfo.GetVlanId()))),
					niselector.WithVlan(itfceInfo.GetVlanId()),
					niselector.WithItfceSelectorKind(itfceInfo.GetItfceSelectorKind()),
					niselector.WithItfceSelectorTags(itfceInfo.GetItfceSelectorTags()),
					niselector.WithBridgeDomainName(itfceInfo.GetBridgeDomainName()),
				)

				si, err := r.createNetworkInstanceSubInterfaces(ctx, cr, nip)
				if err != nil {
					return nil, err
				}
				if err := si.CreateNddaSubInterface(ctx, cr); err != nil {
					return nil, err
				}
				// keep track of the active itfces and nodes per network instance
				activeNiNodeAndLinks = append(activeNiNodeAndLinks, nip)
			}
		}

		// if the tunnel mechanism in the bridge domin is vxlan add the vxlan interface
		if cr.GetBridgeDomainTunnel(bdName) == vpcv1alpha1.TunnelVxlan.String() {
			selectedNodeItfces := getVxlanNodeItfces(bdName, activeNiNodeAndLinks, nddaItfces)
			fmt.Printf("rt selectedNodeItfces: %v\n", selectedNodeItfces)
			for nodeName, itfces := range selectedNodeItfces {
				log.Debug("bridge domain vxlan", "bdName", bdName, "node", nodeName)
				// Although we walk through interfaces we expect only 1 interface per node
				for _, itfceInfo := range itfces {
					log.Debug("bridge domain vxlan",
						"bdName", bdName, "node", nodeName,
						"itfce", itfceInfo.GetItfceName(),
						"vlanid", itfceInfo.GetVlanId())

					// add vxlan in the bridge domain
					nip := niselector.NewItfceInfo(
						niselector.WithRegister(register),
						niselector.WithAddressAllocationStrategy(addressAllocationStrategy),
						niselector.WithResourceClient(registry.RegisterKindIpam.String(), ipamClient),
						niselector.WithResourceClient(registry.RegisterKindAs.String(), aspoolClient),
						niselector.WithResourceClient(registry.RegisterKindNi.String(), niregistryClient),
						niselector.WithEpgName(itfceInfo.GetEpgName()),
						niselector.WithNiName(strings.Join([]string{bdName, infra.NiKindBridged.String()}, "-")),
						niselector.WithNiKind(infra.NiKindBridged.String()),
						niselector.WithNodeName(nodeName),
						niselector.WithItfceName(itfceInfo.GetItfceName()),
						niselector.WithItfceIndex(strconv.Itoa(int(*niIndex))),
						//niselector.WithVlan(*niIndex),
						niselector.WithItfceSelectorKind(itfceInfo.GetItfceSelectorKind()),
						niselector.WithItfceSelectorTags(itfceInfo.GetItfceSelectorTags()),
						niselector.WithBridgeDomainName(itfceInfo.GetBridgeDomainName()),
					)

					si, err := r.createNetworkInstanceSubInterfaces(ctx, cr, nip)
					if err != nil {
						return nil, err
					}
					if err := si.CreateNddaSubInterface(ctx, cr); err != nil {
						return nil, err
					}
					activeNiNodeAndLinks = append(activeNiNodeAndLinks, nip)
				}
			}
		}
	}
	// find and register the route tables in the ni pool/register
	for rtName := range cr.GetRoutingTables() {

		nip := niselector.NewItfceInfo(
			niselector.WithResourceClient(registry.RegisterKindIpam.String(), ipamClient),
			niselector.WithResourceClient(registry.RegisterKindAs.String(), aspoolClient),
			niselector.WithResourceClient(registry.RegisterKindNi.String(), niregistryClient),
			niselector.WithNiName(strings.Join([]string{rtName, infra.NiKindRouted.String()}, "-")),
			niselector.WithNiKind(infra.NiKindRouted.String()),
		)
		ni, err := r.createGlobalNetworkInstance(ctx, cr, nip)
		if err != nil {
			return nil, err
		}

		niOptions := &infra.NiOptions{
			RegistryName:        niRegistryName,
			NetworkInstanceName: nip.GetNiName(),
		}
		niIndex, err := ni.GrpcAllocateNiIndex(ctx, cr, niOptions)
		if err != nil {
			return nil, err
		}
		ni.SetIndex(niIndex)

		// get the epg, nodeitfce selectors belonging to this bridge domain
		epgSelectors, nodeItfceSelectors, err := cr.GetRoutingTableEpgAndNodeItfceSelectors(rtName)
		if err != nil {
			return nil, err
		}
		//log.Debug("routed epgSelectors", "epgSelectors", epgSelectors)
		//log.Debug("routed nodeItfceSelectors", "nodeItfceSelectors", nodeItfceSelectors)
		// get the nodes and interfaces based on the epg, nodeitfce selectors
		selectedNodeItfces := getSelectedNodeItfces(epgSelectors, nodeItfceSelectors, nddaItfces)
		//fmt.Printf("rt selectedNodeItfces: %v\n", selectedNodeItfces)
		for nodeName, itfces := range selectedNodeItfces {
			log.Debug("route table interfaces", "rTable", rtName, "node", nodeName)
			for _, itfceInfo := range itfces {
				log.Debug("route table interface",
					"rTable", rtName, "node", nodeName,
					"itfce", itfceInfo.GetItfceName(),
					"vlanid", itfceInfo.GetVlanId())

				nip := niselector.NewItfceInfo(
					niselector.WithRegister(register),
					niselector.WithAddressAllocationStrategy(addressAllocationStrategy),
					niselector.WithResourceClient(registry.RegisterKindIpam.String(), ipamClient),
					niselector.WithResourceClient(registry.RegisterKindAs.String(), aspoolClient),
					niselector.WithResourceClient(registry.RegisterKindNi.String(), niregistryClient),
					niselector.WithEpgName(itfceInfo.GetEpgName()),
					niselector.WithNiName(strings.Join([]string{rtName, infra.NiKindRouted.String()}, "-")),
					niselector.WithNiKind(infra.NiKindRouted.String()),
					niselector.WithNodeName(nodeName),
					niselector.WithItfceName(itfceInfo.GetItfceName()),
					niselector.WithItfceIndex(strconv.Itoa(int(itfceInfo.GetVlanId()))),
					niselector.WithVlan(itfceInfo.GetVlanId()),
					niselector.WithIpv4Prefixes(itfceInfo.GetIpv4Prefixes()),
					niselector.WithIpv6Prefixes(itfceInfo.GetIpv6Prefixes()),
					niselector.WithItfceSelectorKind(itfceInfo.GetItfceSelectorKind()),
					niselector.WithItfceSelectorTags(itfceInfo.GetItfceSelectorTags()),
					niselector.WithBridgeDomainName(itfceInfo.GetBridgeDomainName()),
				)

				si, err := r.createNetworkInstanceSubInterfaces(ctx, cr, nip)
				if err != nil {
					return nil, err
				}

				if err := si.CreateNddaSubInterface(ctx, cr); err != nil {
					return nil, err
				}
				activeNiNodeAndLinks = append(activeNiNodeAndLinks, nip)
			}
		}

		// if the tunnel mechanism in the routing table is vxlan add the vxlan interface
		if cr.GetRoutingTableTunnel(rtName) == vpcv1alpha1.TunnelVxlan.String() {
			selectedNodeItfces := getVxlanNodeItfces(rtName, activeNiNodeAndLinks, nddaItfces)
			//fmt.Printf("rt selectedNodeItfces: %v\n", selectedNodeItfces)
			for nodeName, itfces := range selectedNodeItfces {
				log.Debug("route table vxlan", "rTable", rtName, "node", nodeName)
				for _, itfceInfo := range itfces {
					log.Debug("route table vxlan",
						"rTable", rtName, "node", nodeName,
						"itfce", itfceInfo.GetItfceName(),
						"vlanid", itfceInfo.GetVlanId())

					// add vxlan in the routing table
					nip := niselector.NewItfceInfo(
						niselector.WithRegister(register),
						niselector.WithAddressAllocationStrategy(addressAllocationStrategy),
						niselector.WithResourceClient(registry.RegisterKindIpam.String(), ipamClient),
						niselector.WithResourceClient(registry.RegisterKindAs.String(), aspoolClient),
						niselector.WithResourceClient(registry.RegisterKindNi.String(), niregistryClient),
						niselector.WithEpgName(itfceInfo.GetEpgName()),
						niselector.WithNiName(strings.Join([]string{rtName, infra.NiKindRouted.String()}, "-")),
						niselector.WithNiKind(infra.NiKindRouted.String()),
						niselector.WithNodeName(nodeName),
						niselector.WithItfceName(itfceInfo.GetItfceName()),
						niselector.WithItfceIndex(strconv.Itoa(int(*niIndex))), // TODO need to get a better strategy
						//niselector.WithVlan(itfceInfo.GetVlanId()+5),                          // TODO need to get a better strategy
						niselector.WithIpv4Prefixes(itfceInfo.GetIpv4Prefixes()),
						niselector.WithIpv6Prefixes(itfceInfo.GetIpv6Prefixes()),
						niselector.WithItfceSelectorKind(itfceInfo.GetItfceSelectorKind()),
						niselector.WithItfceSelectorTags(itfceInfo.GetItfceSelectorTags()),
						niselector.WithBridgeDomainName(itfceInfo.GetBridgeDomainName()),
					)

					si, err := r.createNetworkInstanceSubInterfaces(ctx, cr, nip)
					if err != nil {
						return nil, err
					}
					if err := si.CreateNddaSubInterface(ctx, cr); err != nil {
						return nil, err
					}
					activeNiNodeAndLinks = append(activeNiNodeAndLinks, nip)
				}
			}
		}
	}

	// handle irb
	for rtName, bds := range cr.GetBridge2RoutingTable() {
		//rtni, err := r.getGlobalNetworkInstance(crName, strings.Join([]string{rtName, infra.NiKindRouted.String()}, "."))
		//if err != nil {
		//	return nil, err
		//}
		for _, bd := range bds {
			// validate if the bridge domain exists
			bdfound := false
			for bdName := range cr.GetBridgeDomains() {
				if bd.GetName() == bdName {
					bdfound = true
					break
				}
			}
			if bdfound {
				// find the nodes on which the bridgedomain is active
				selectedNodeItfces := getIrbNodeItfces(bd, activeNiNodeAndLinks, nddaItfces)
				fmt.Printf("rt selectedNodeItfces: %v\n", selectedNodeItfces)
				for nodeName, itfces := range selectedNodeItfces {
					log.Debug("route table irb", "rTable", rtName, "node", nodeName)

					for _, itfceInfo := range itfces {
						log.Debug("route table irb",
							"rTable", rtName, "node", nodeName,
							"itfce", itfceInfo.GetItfceName(),
							"vlanid", itfceInfo.GetVlanId())

						bdni, err := r.getGlobalNetworkInstance(getCrName(cr), strings.Join([]string{bd.GetName(), infra.NiKindBridged.String()}, "-"))
						if err != nil {
							return nil, err
						}

						// add irb in the routing table
						nip := niselector.NewItfceInfo(
							niselector.WithRegister(register),
							niselector.WithAddressAllocationStrategy(addressAllocationStrategy),
							niselector.WithResourceClient(registry.RegisterKindIpam.String(), ipamClient),
							niselector.WithResourceClient(registry.RegisterKindAs.String(), aspoolClient),
							niselector.WithResourceClient(registry.RegisterKindNi.String(), niregistryClient),
							niselector.WithEpgName(itfceInfo.GetEpgName()),
							niselector.WithNiName(strings.Join([]string{rtName, infra.NiKindRouted.String()}, "-")),
							niselector.WithNiKind(infra.NiKindRouted.String()),
							niselector.WithNodeName(nodeName),
							niselector.WithItfceName(itfceInfo.GetItfceName()),
							niselector.WithItfceIndex(strconv.Itoa(int(bdni.GetIndex()+10000))),
							//niselector.WithVlan(itfceInfo.GetVlanId()+5),
							niselector.WithIpv4Prefixes(itfceInfo.GetIpv4Prefixes()),
							niselector.WithIpv6Prefixes(itfceInfo.GetIpv6Prefixes()),
							niselector.WithItfceSelectorKind(itfceInfo.GetItfceSelectorKind()),
							niselector.WithItfceSelectorTags(itfceInfo.GetItfceSelectorTags()),
							niselector.WithBridgeDomainName(itfceInfo.GetBridgeDomainName()),
						)

						si, err := r.createNetworkInstanceSubInterfaces(ctx, cr, nip)
						if err != nil {
							return nil, err
						}
						if err := si.CreateNddaSubInterface(ctx, cr); err != nil {
							return nil, err
						}
						activeNiNodeAndLinks = append(activeNiNodeAndLinks, nip)

						log.Debug("bridge domain irb",
							"bdName", bd.GetName(), "node", nodeName,
							"itfce", itfceInfo.GetItfceName(),
							"vlanid", itfceInfo.GetVlanId())

						// add irb in the bridge table
						nip = niselector.NewItfceInfo(
							niselector.WithRegister(register),
							niselector.WithAddressAllocationStrategy(addressAllocationStrategy),
							niselector.WithResourceClient(registry.RegisterKindIpam.String(), ipamClient),
							niselector.WithResourceClient(registry.RegisterKindAs.String(), aspoolClient),
							niselector.WithResourceClient(registry.RegisterKindNi.String(), niregistryClient),
							niselector.WithEpgName(itfceInfo.GetEpgName()),
							niselector.WithNiName(strings.Join([]string{bd.GetName(), infra.NiKindBridged.String()}, "-")),
							niselector.WithNiKind(infra.NiKindBridged.String()),
							niselector.WithNodeName(nodeName),
							niselector.WithItfceName(itfceInfo.GetItfceName()),
							niselector.WithItfceIndex(strconv.Itoa(int(bdni.GetIndex()))),
							//niselector.WithVlan(itfceInfo.GetVlanId()),
							niselector.WithItfceSelectorKind(itfceInfo.GetItfceSelectorKind()),
							niselector.WithItfceSelectorTags(itfceInfo.GetItfceSelectorTags()),
							niselector.WithBridgeDomainName(itfceInfo.GetBridgeDomainName()),
						)

						si, err = r.createNetworkInstanceSubInterfaces(ctx, cr, nip)
						if err != nil {
							return nil, err
						}
						if err := si.CreateNddaSubInterface(ctx, cr); err != nil {
							return nil, err
						}
						activeNiNodeAndLinks = append(activeNiNodeAndLinks, nip)
					}
				}
			}
		}
	}

	// find the nodes and interfaces
	if err := r.validateBackend(ctx, cr, ipamName, niRegistryName, activeNiNodeAndLinks); err != nil {
		return nil, err
	}

	// create the network instance, since we have up to date info it is better to
	// wait for NI creation at this stage
	nodes := r.handler.GetInfraNodes(getCrName(cr))

	for _, n := range nodes {
		for niName, ni := range n.GetNis() {
			// register ni
			niOptions := &infra.NiOptions{
				RegistryName:        niRegistryName,
				NetworkInstanceName: niName,
			}
			if err := ni.CreateNiRegister(ctx, cr, niOptions); err != nil {
				return nil, err
			}
			// create ni in ipam; only for routed network instances
			if ni.GetKind() == infra.NiKindRouted.String() {
				// we ensure the name is common to avoid creating an instance per node
				ipamOptions := &infra.IpamOptions{
					RegistryName:        ipamName,
					NetworkInstanceName: niName,
				}
				r.log.Debug("ni routed creation/registration", "ipamOptions 1", *ipamOptions)
				if err := ni.CreateIpamNi(ctx, cr, ipamOptions); err != nil {
					return nil, err
				}
				for _, si := range ni.GetSubInterfaces() {
					r.log.Debug("ipam create prefix", "name", si.GetInterfaceSubInterfaceName(), "ipv4", si.GetAddressesInfo(ipamv1alpha1.AddressFamilyIpv4.String()),
						"ipv6", si.GetAddressesInfo(ipamv1alpha1.AddressFamilyIpv6.String()))
					for prefix, ai := range si.GetAddressesInfo(ipamv1alpha1.AddressFamilyIpv4.String()) {
						r.log.Debug("ipam create prefix", "prefix", prefix, "af", ipamv1alpha1.AddressFamilyIpv4.String())

						ipamOptions.AddressFamily = ipamv1alpha1.AddressFamilyIpv4.String()
						/*
							ipamOptions := &infra.IpamOptions{
								RegistryName:        ipamName,
								NetworkInstanceName: niName,
								AddressFamily:       ipamv1alpha1.AddressFamilyIpv4.String(),
							}
						*/
						r.log.Debug("ni routed creation/registration", "ipamOptions 2", *ipamOptions)
						if err := ai.CreateIpamPrefix(ctx, cr, ipamOptions); err != nil {
							return nil, err
						}

						if p, err := ai.GrpcAllocateEndpointIP(ctx, cr, ipamOptions); err != nil {
							return nil, err
						} else {
							r.log.Debug("GRPC addess", "address", *p)
						}
					}
					for prefix, ai := range si.GetAddressesInfo(ipamv1alpha1.AddressFamilyIpv6.String()) {
						r.log.Debug("ipam create prefix", "prefix", prefix, "af", ipamv1alpha1.AddressFamilyIpv6.String())

						ipamOptions.AddressFamily = ipamv1alpha1.AddressFamilyIpv6.String()
						/*
							ipamOptions := &infra.IpamOptions{
								RegistryName:        ipamName,
								NetworkInstanceName: niName,
								AddressFamily:       ipamv1alpha1.AddressFamilyIpv6.String(),
							}
						*/
						r.log.Debug("ni routed creation/registration", "ipamOptions 3", *ipamOptions)
						if err := ai.CreateIpamPrefix(ctx, cr, ipamOptions); err != nil {
							return nil, err
						}

						if p, err := ai.GrpcAllocateEndpointIP(ctx, cr, ipamOptions); err != nil {
							return nil, err
						} else {
							r.log.Debug("GRPC addess", "address", *p)
						}
					}
				}
			}
			// create ni in adaptor layer
			if err := ni.CreateNddaNi(ctx, cr); err != nil {
				return nil, err
			}
		}
	}
	cr.SetOrganization(cr.GetOrganization())
	cr.SetDeployment(cr.GetDeployment())
	cr.SetAvailabilityZone(cr.GetAvailabilityZone())
	cr.SetVpcName(cr.GetVpcName())

	if err := r.getNddaResources(ctx, cr); err != nil {
		return nil, err
	}

	return nil, nil

}

func (r *application) validateBackend(ctx context.Context, cr vpcv1alpha1.Vp, ipamName, niRegistryName string, activeNiNodeLinks []niselector.ItfceInfo) error {

	nodes := r.handler.GetInfraNodes(getCrName(cr))
	// keeps track of the active nodes
	activeNodes := make(map[string]bool)
	// walk over the backend data (node/interface/subinterface) and validate
	// if the interfaces and subinterfaces are still active, if not delete them
	// keep track of the active nodes
	for nodeName, node := range nodes {
		for itfceName, itfce := range node.GetInterfaces() {
			itfceFound := false
			for siIndex, si := range itfce.GetSubInterfaces() {
				siFound := false
				for _, nip := range activeNiNodeLinks {
					if nodeName == nip.GetNodeName() && itfceName == nip.GetItfceName() && siIndex == nip.GetItfceIndex() {
						siFound = true
						itfceFound = true
						activeNodes[nodeName] = true
						break
					}
				}
				if !siFound {
					for _, ai := range si.GetAddressesInfo(ipamv1alpha1.AddressFamilyIpv4.String()) {
						if err := ai.DeleteIpamPrefix(ctx, cr, &infra.IpamOptions{
							RegistryName: ipamName,
							//NetworkInstanceName: ai.GetSubInterface().GetNi().GetName(),
							AddressFamily: ipamv1alpha1.AddressFamilyIpv4.String(),
						}); err != nil {
							return err
						}
					}
					for _, ai := range si.GetAddressesInfo(ipamv1alpha1.AddressFamilyIpv6.String()) {
						if err := ai.DeleteIpamPrefix(ctx, cr, &infra.IpamOptions{
							RegistryName: ipamName,
							//NetworkInstanceName: ai.GetSubInterface().GetNi().GetName(),
							AddressFamily: ipamv1alpha1.AddressFamilyIpv6.String(),
						}); err != nil {
							return err
						}
					}

					if err := si.DeleteNddaSubInterface(ctx, cr); err != nil {
						return err
					}

					// delete subinterface from backend
					delete(node.GetInterfaces()[itfceName].GetSubInterfaces(), siIndex)
				}
			}
			if !itfceFound {
				// delete interface from backend
				delete(node.GetInterfaces(), itfceName)
			}
		}
	}
	// walk over the backend data (node/ni/subinterface) and validate
	// if the nis and subinterfaces are still active, if not delete them
	for nodeName, node := range nodes {
		for niName, ni := range node.GetNis() {
			niFound := false
			for _, si := range ni.GetSubInterfaces() {
				siFound := false
				for _, nip := range activeNiNodeLinks {
					if nodeName == nip.GetNodeName() && niName == nip.GetNiName() && si.GetIndex() == nip.GetItfceIndex() && si.GetInterface().GetName() == nip.GetItfceName() {
						siFound = true
						niFound = true
						break
					}
				}
				if !siFound {
					// delete subinterface from backend
					for _, ai := range si.GetAddressesInfo(ipamv1alpha1.AddressFamilyIpv4.String()) {
						if err := ai.DeleteIpamPrefix(ctx, cr, &infra.IpamOptions{
							RegistryName: ipamName,
							//NetworkInstanceName: ai.GetSubInterface().GetNi().GetName(),
							AddressFamily: ipamv1alpha1.AddressFamilyIpv4.String(),
						}); err != nil {
							return err
						}
					}
					for _, ai := range si.GetAddressesInfo(ipamv1alpha1.AddressFamilyIpv6.String()) {
						if err := ai.DeleteIpamPrefix(ctx, cr, &infra.IpamOptions{
							RegistryName: ipamName,
							//NetworkInstanceName: ai.GetSubInterface().GetNi().GetName(),
							AddressFamily: ipamv1alpha1.AddressFamilyIpv6.String(),
						}); err != nil {
							return err
						}
					}
					//delete(node.GetNis()[niName].GetSubInterfaces(), subItfceName)
					ni.DeleteSubInterface(si, si.GetInterface())
				}
			}
			if !niFound {
				// delete subinterface from backend
				delete(node.GetInterfaces(), niName)
			}
		}
	}

	// validate if the nodes are still active if not delete them
	for nodeName := range nodes {
		nodeFound := false
		for activeNodeName := range activeNodes {
			if nodeName == activeNodeName {
				nodeFound = true
				break
			}
		}
		if !nodeFound {
			// Interfaces shoudl not be deleted since they are not created here -> epgs take care of that

			for niName, ni := range nodes[nodeName].GetNis() {
				// delete ni in ipam only for routed network instances
				if ni.GetKind() == infra.NiKindRouted.String() {
					if err := ni.DeleteIpamNi(ctx, cr, &infra.IpamOptions{
						RegistryName:        ipamName,
						NetworkInstanceName: niName,
					}); err != nil {
						return err
					}
					for _, si := range ni.GetSubInterfaces() {
						for _, ai := range si.GetAddressesInfo("ipv4") {
							if err := ai.DeleteIpamPrefix(ctx, cr, &infra.IpamOptions{
								RegistryName:        ipamName,
								NetworkInstanceName: niName,
								AddressFamily:       ipamv1alpha1.AddressFamilyIpv4.String(),
							}); err != nil {
								return nil
							}
						}
						for _, ai := range si.GetAddressesInfo("ipv6") {
							if err := ai.DeleteIpamPrefix(ctx, cr, &infra.IpamOptions{
								RegistryName:        ipamName,
								NetworkInstanceName: niName,
								AddressFamily:       ipamv1alpha1.AddressFamilyIpv6.String(),
							}); err != nil {
								return nil
							}
						}
					}
				}
				// delete ni in adaptor layer
				if err := ni.DeleteNddaNi(ctx, cr); err != nil {
					return err
				}
				niOptions := &infra.NiOptions{
					RegistryName:        niRegistryName,
					NetworkInstanceName: niName,
				}
				if err := ni.DeleteNiRegister(ctx, cr, niOptions); err != nil {
					return err
				}
			}

			// delete node from backend
			delete(nodes, nodeName)
		}
	}

	return nil
}

func (r *application) getNddaResources(ctx context.Context, cr vpcv1alpha1.Vp) error {
	/*
		selector := labels.NewSelector()
		req, err := labels.NewRequirement(networkv1alpha1.LabelNddaOwner,
			selection.In,
			[]string{odns.GetOdnsResourceKindName(cr.GetName(), strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind))})
		if err != nil {
			r.log.Debug("wrong object", "Error", err)
			return err
		}
		selector = selector.Add(*req)
	*/

	opts := []client.ListOption{
		client.MatchingLabels{networkv1alpha1.LabelNddaOwner: odns.GetOdnsResourceKindName(cr.GetName(), strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind))},
	}

	itfce := r.newNddaItfceList()
	if err := r.client.List(ctx, itfce, opts...); err != nil {
		return err
	}

	for _, nddaIf := range itfce.GetInterfaces() {
		fmt.Printf("nddaIf: %s, info: %s\n", nddaIf.GetInterfaceName(), nddaIf.GetName())
	}

	si := r.newNddaSubInterfaceList()
	if err := r.client.List(ctx, si, opts...); err != nil {
		return err
	}

	for _, nddaSi := range si.GetSubInterfaces() {
		fmt.Printf("nddaSi: %s, info: %s\n", nddaSi.GetInterfaceName(), nddaSi.GetName())
	}

	ni := r.newNddaNiList()
	if err := r.client.List(ctx, ni, opts...); err != nil {
		return err
	}

	for _, nddaNi := range ni.GetNetworkInstance() {
		fmt.Printf("nddaNi: %s, info: %s\n", nddaNi.GetNodeName(), nddaNi.GetName())
	}

	return nil
}
