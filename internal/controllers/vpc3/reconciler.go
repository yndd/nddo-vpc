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

package vpc3

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/yndd/ndd-runtime/pkg/event"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/ndd-runtime/pkg/utils"
	networkv1alpha1 "github.com/yndd/ndda-network/apis/network/v1alpha1"
	"github.com/yndd/ndda-network/pkg/ndda/ndda"
	"github.com/yndd/ndda-network/pkg/ndda/niinfo"
	"github.com/yndd/nddo-runtime/pkg/reconciler/managed"
	"github.com/yndd/nddo-runtime/pkg/resource"
	vpcv1alpha1 "github.com/yndd/nddo-vpc/apis/vpc/v1alpha1"
	"github.com/yndd/nddo-vpc/internal/networkhandler"
	"github.com/yndd/nddo-vpc/internal/shared"
	"github.com/yndd/nddo-vpc/internal/speedyhandler"
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
	nddaiflfn := func() networkv1alpha1.IFNetworkInterfaceList { return &networkv1alpha1.NetworkInterfaceList{} }
	nddasilfn := func() networkv1alpha1.IFNetworkInterfaceSubinterfaceList {
		return &networkv1alpha1.NetworkInterfaceSubinterfaceList{}
	}
	nddanilfn := func() networkv1alpha1.IFNetworkNetworkInstanceList {
		return &networkv1alpha1.NetworkNetworkInstanceList{}
	}

	shandler := speedyhandler.New()

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
			//handler:                 nddcopts.SchemaHandler,
			registry: nddcopts.Registry,
			nddaHandler: ndda.New(
				ndda.WithClient(resource.ClientApplicator{
					Client:     mgr.GetClient(),
					Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
				}),
				ndda.WithLogger(nddcopts.Logger.WithValues("nddahandler", name)),
			),
			networkHandler: networkhandler.New(
				networkhandler.WithClient(resource.ClientApplicator{
					Client:     mgr.GetClient(),
					Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
				}),
				networkhandler.WithLogger(nddcopts.Logger.WithValues("networkhandler", name)),
			),
			speedyHandler: shandler,
		}),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	nddaItfceHandler := &EnqueueRequestForAllNddaInterfaces{
		client:        mgr.GetClient(),
		log:           nddcopts.Logger,
		ctx:           context.Background(),
		newVpcList:    vpclfn,
		speedyHandler: shandler,
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o).
		For(&vpcv1alpha1.Vpc{}).
		Owns(&vpcv1alpha1.Vpc{}).
		WithEventFilter(resource.IgnoreUpdateWithoutGenerationChangePredicate()).
		Watches(&source.Kind{Type: &networkv1alpha1.NetworkInterface{}}, nddaItfceHandler).
		WithEventFilter(resource.IgnoreUpdateWithoutGenerationChangePredicate()).
		Complete(r)

}

type application struct {
	client resource.ClientApplicator
	log    logging.Logger

	newVpc                  func() vpcv1alpha1.Vp
	newVpcList              func() vpcv1alpha1.VpList
	newNddaItfceList        func() networkv1alpha1.IFNetworkInterfaceList
	newNddaSubInterfaceList func() networkv1alpha1.IFNetworkInterfaceSubinterfaceList
	newNddaNiList           func() networkv1alpha1.IFNetworkNetworkInstanceList

	registry       registry.Registry
	nddaHandler    ndda.Handler
	networkHandler networkhandler.Handler
	speedyHandler  speedyhandler.Handler
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
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return nil, errors.New(errUnexpectedResource)
	}

	r.speedyHandler.Init(getCrName(mg))

	//return r.handleAppLogic(ctx, cr)
	info, err := r.populateSchema(ctx, mg)
	if err != nil {
		return info, err
	}

	if err := r.networkHandler.ValidateSchema(ctx, mg); err != nil {
		return nil, err
	}
	labels := make(map[string]string)
	if err := r.networkHandler.DeploySchema(ctx, mg, labels); err != nil {
		return nil, err
	}

	cr.SetOrganization(cr.GetOrganization())
	cr.SetDeployment(cr.GetDeployment())
	cr.SetAvailabilityZone(cr.GetAvailabilityZone())

	return nil, nil
}

func (r *application) FinalUpdate(ctx context.Context, mg resource.Managed) {
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return
	}
	crName := getCrName(cr)
	r.networkHandler.PrintDevices(crName)

	r.networkHandler.DestroySchema(getCrName(mg))
}

func (r *application) Timeout(ctx context.Context, mg resource.Managed) time.Duration {
	crName := getCrName(mg)
	speedy := r.speedyHandler.GetSpeedy(crName)
	if speedy <= 2 {
		r.speedyHandler.IncrementSpeedy(crName)
		r.log.Debug("Speedy incr", "number", r.speedyHandler.GetSpeedy(crName))
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
	_, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return
	}
	crName := getCrName(mg)
	r.networkHandler.DestroySchema(crName)
	r.speedyHandler.Delete(crName)
}

func (r *application) populateSchema(ctx context.Context, mg resource.Managed) (map[string]string, error) {
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return nil, errors.New(errUnexpectedResource)
	}

	// initialize the schema elements
	s := r.networkHandler.InitSchema(getCrName(mg))

	addressAllocationStrategy, err := r.registry.GetAddressAllocationStrategy(ctx, mg)
	if err != nil {
		return nil, err
	}
	// get AS -> depending on if this is per org or per deployment

	for bdName := range cr.GetBridgeDomains() {
		// get AS
		// allocate RT
		// get the epg, nodeitfce selectors belonging to this bridge domain
		// this validates and transforms the user input
		epgSelectors, nodeItfceSelectors, err := cr.GetBridgeEpgAndNodeItfceSelectors(bdName)
		if err != nil {
			return nil, err
		}
		r.log.Debug("bdName info", "epgSelectors", epgSelectors, "nodeItfceSelectors", nodeItfceSelectors)
		// get the nodes and interfaces based on the epg, nodeitfce selectors
		selectedNodeItfces, err := r.nddaHandler.GetSelectedNodeItfces(mg, epgSelectors, nodeItfceSelectors)
		if err != nil {
			return nil, err
		}
		r.log.Debug("bdName info", "selectedNodeItfces", selectedNodeItfces)
		for deviceName, itfces := range selectedNodeItfces {
			for _, itfceInfo := range itfces {
				niInfo := &niinfo.NiInfo{
					Name:  utils.StringPtr(bdName),
					Index: utils.Uint32Ptr(hash(strings.Join([]string{bdName, string(networkv1alpha1.E_NetworkInstanceKind_BRIDGED)}, "-"))), // to do allocation
					Kind:  networkv1alpha1.E_NetworkInstanceKind_BRIDGED,
				}
				r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)

			}
		}

		// if the tunnel mechanism in the bridge domin is vxlan add the vxlan interface
		if cr.GetBridgeDomainTunnel(bdName) == vpcv1alpha1.TunnelVxlan.String() {
			selectedNodeItfces, err := r.nddaHandler.GetSelectedNodeItfcesVxlan(mg, s, bdName)
			if err != nil {
				return nil, err
			}
			for deviceName, itfces := range selectedNodeItfces {
				for _, itfceInfo := range itfces {
					niInfo := &niinfo.NiInfo{
						Name:  utils.StringPtr(bdName),
						Index: utils.Uint32Ptr(hash(strings.Join([]string{bdName, string(networkv1alpha1.E_NetworkInstanceKind_BRIDGED)}, "-"))),
						Kind:  networkv1alpha1.E_NetworkInstanceKind_BRIDGED,
					}
					r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)
				}
			}
		}
	}

	for rtName := range cr.GetRoutingTables() {
		// allocate RT
		// get the epg, nodeitfce selectors belonging to this bridge domain
		epgSelectors, nodeItfceSelectors, err := cr.GetRoutingTableEpgAndNodeItfceSelectors(rtName)
		if err != nil {
			return nil, err
		}
		//log.Debug("routed epgSelectors", "epgSelectors", epgSelectors)
		//log.Debug("routed nodeItfceSelectors", "nodeItfceSelectors", nodeItfceSelectors)
		// get the nodes and interfaces based on the epg, nodeitfce selectors
		selectedNodeItfces, err := r.nddaHandler.GetSelectedNodeItfces(mg, epgSelectors, nodeItfceSelectors)
		if err != nil {
			return nil, err
		}
		for deviceName, itfces := range selectedNodeItfces {
			for _, itfceInfo := range itfces {

				niInfo := &niinfo.NiInfo{
					Name:  utils.StringPtr(rtName),
					Index: utils.Uint32Ptr(hash(strings.Join([]string{rtName, string(networkv1alpha1.E_NetworkInstanceKind_ROUTED)}, "-"))),
					Kind:  networkv1alpha1.E_NetworkInstanceKind_ROUTED,
				}
				r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)
			}
		}
		// if the tunnel mechanism in the bridge domin is vxlan add the vxlan interface
		if cr.GetBridgeDomainTunnel(rtName) == vpcv1alpha1.TunnelVxlan.String() {
			selectedNodeItfces, err := r.nddaHandler.GetSelectedNodeItfcesVxlan(mg, s, rtName)
			if err != nil {
				return nil, err
			}
			for deviceName, itfces := range selectedNodeItfces {
				for _, itfceInfo := range itfces {
					niInfo := &niinfo.NiInfo{
						Name:  utils.StringPtr(rtName),
						Index: utils.Uint32Ptr(hash(strings.Join([]string{rtName, string(networkv1alpha1.E_NetworkInstanceKind_ROUTED)}, "-"))),
						Kind:  networkv1alpha1.E_NetworkInstanceKind_ROUTED,
					}
					r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)
				}
			}
		}
	}
	// handle irb
	for rtName, bds := range cr.GetBridge2RoutingTable() {
		for _, bd := range bds {
			bdfound := false
			var bdName string
			for bridgeDomainName := range cr.GetBridgeDomains() {
				if bd.GetName() == bridgeDomainName {
					bdfound = true
					bdName = bd.GetName()
					break
				}
			}
			if bdfound {
				selectedNodeItfces, err := r.nddaHandler.GetSelectedNodeItfcesIrb(mg, s, rtName)
				if err != nil {
					return nil, err
				}
				for deviceName, itfces := range selectedNodeItfces {
					for _, itfceInfo := range itfces {
						niInfo := &niinfo.NiInfo{
							Name:  utils.StringPtr(bdName),
							Index: utils.Uint32Ptr(hash(strings.Join([]string{rtName, string(networkv1alpha1.E_NetworkInstanceKind_BRIDGED)}, "-"))),
							Kind:  networkv1alpha1.E_NetworkInstanceKind_BRIDGED,
						}
						r.PopulateSchema(ctx, cr, deviceName, itfceInfo, niInfo, addressAllocationStrategy)

						// the ipv4/ipv6 prefix info comes from the bd interface
						itfceInfo.SetIpv4Prefixes(bd.GetIPv4Prefixes())
						itfceInfo.SetIpv6Prefixes(bd.GetIPv6Prefixes())
						niInfo = &niinfo.NiInfo{
							Name:  utils.StringPtr(rtName),
							Index: utils.Uint32Ptr(hash(strings.Join([]string{rtName, string(networkv1alpha1.E_NetworkInstanceKind_ROUTED)}, "-"))),
							Kind:  networkv1alpha1.E_NetworkInstanceKind_ROUTED,
						}
						r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)
					}
				}
			}
		}
	}

	return nil, nil
}
