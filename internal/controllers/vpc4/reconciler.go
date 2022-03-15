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

package vpc4

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/yndd/ndd-runtime/pkg/event"
	"github.com/yndd/ndd-runtime/pkg/logging"

	networkv1alpha1 "github.com/yndd/ndda-network/apis/network/v1alpha1"
	"github.com/yndd/ndda-network/pkg/ndda/itfceinfo"
	"github.com/yndd/ndda-network/pkg/ndda/niinfo"
	"github.com/yndd/nddo-runtime/pkg/reconciler/managed"
	"github.com/yndd/nddo-runtime/pkg/resource"
	vpcv1alpha1 "github.com/yndd/nddo-vpc/apis/vpc/v1alpha1"
	"github.com/yndd/nddo-vpc/internal/nihandler"
	"github.com/yndd/nddo-vpc/internal/shared"
	"github.com/yndd/nddo-vpc/internal/speedyhandler"
	"github.com/yndd/nddo-vpc/internal/srl3handler"
	srlv1alpha1 "github.com/yndd/nddp-srl3/apis/srl3/v1alpha1"
	srlndda "github.com/yndd/nddp-srl3/pkg/ndda"
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
	//nddaiflfn := func() networkv1alpha1.IFNetworkInterfaceList { return &networkv1alpha1.NetworkInterfaceList{} }
	//nddasilfn := func() networkv1alpha1.IFNetworkInterfaceSubinterfaceList {
	//	return &networkv1alpha1.NetworkInterfaceSubinterfaceList{}
	//}
	//nddanilfn := func() networkv1alpha1.IFNetworkNetworkInstanceList {
	//	return &networkv1alpha1.NetworkNetworkInstanceList{}
	//}

	shandler := speedyhandler.New()

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(vpcv1alpha1.VpcGroupVersionKind),
		managed.WithLogger(nddcopts.Logger.WithValues("controller", name)),
		managed.WithApplication(&application{
			client: resource.ClientApplicator{
				Client:     mgr.GetClient(),
				Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
			},
			log:        nddcopts.Logger.WithValues("applogic", name),
			newVpc:     vpcfn,
			newVpcList: vpclfn,
			//newNddaSrlItfceList:        nddaiflfn,
			//newNddaSrlSubInterfaceList: nddasilfn,
			//newNddaSrlNiList:           nddanilfn,
			//handler:                 nddcopts.SchemaHandler,
			registry: nddcopts.Registry,
			srlNddaHandler: srlndda.New(
				srlndda.WithClient(resource.ClientApplicator{
					Client:     mgr.GetClient(),
					Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
				}),
				srlndda.WithLogger(nddcopts.Logger.WithValues("nddasrlhandler", name)),
			),
			srlHandler: srl3handler.New(
				srl3handler.WithClient(resource.ClientApplicator{
					Client:     mgr.GetClient(),
					Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
				}),
				srl3handler.WithLogger(nddcopts.Logger.WithValues("srlhandler", name)),
			),
			niHandler: nihandler.New(
				nihandler.WithClient(resource.ClientApplicator{
					Client:     mgr.GetClient(),
					Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
				}),
				nihandler.WithLogger(nddcopts.Logger.WithValues("nihandler", name)),
			),
			speedyHandler: shandler,
		}),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	nddaSrlDeviceHandler := &EnqueueRequestForAllNddaSrlDevices{
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
		Watches(&source.Kind{Type: &srlv1alpha1.Srl3Device{}}, nddaSrlDeviceHandler).
		WithEventFilter(resource.IgnoreUpdateWithoutGenerationChangePredicate()).
		Complete(r)

}

type application struct {
	client resource.ClientApplicator
	log    logging.Logger

	newVpc     func() vpcv1alpha1.Vp
	newVpcList func() vpcv1alpha1.VpList
	//newNddaItfceList        func() networkv1alpha1.IFNetworkInterfaceList
	//newNddaSubInterfaceList func() networkv1alpha1.IFNetworkInterfaceSubinterfaceList
	//newNddaNiList           func() networkv1alpha1.IFNetworkNetworkInstanceList

	registry       registry.Registry
	srlNddaHandler srlndda.Handler
	srlHandler     srl3handler.Handler
	speedyHandler  speedyhandler.Handler
	niHandler      nihandler.Handler
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

	if err := r.srlHandler.Validate(ctx, mg); err != nil {
		return nil, err
	}
	labels := make(map[string]string)
	if err := r.srlHandler.Deploy(ctx, mg, labels); err != nil {
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
	r.srlHandler.Print(crName)

	r.srlHandler.Destroy(getCrName(mg))
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

	r.speedyHandler.Init(getCrName(mg))

	//return r.handleAppLogic(ctx, cr)
	_, err := r.populateSchema(ctx, mg)
	if err != nil {
		return false, err
	}
	/*
		if err := r.srlHandler.Validate(ctx, mg); err != nil {
			return false, err
		}
	*/
	/*
		labels := make(map[string]string)
		if err := r.srlHandler.Destroy(ctx, mg, labels); err != nil {
			return false, err
		}
	*/
	return true, nil
}

func (r *application) FinalDelete(ctx context.Context, mg resource.Managed) {
	_, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return
	}
	crName := getCrName(mg)
	r.srlHandler.Destroy(crName)
	r.speedyHandler.Delete(crName)
}

func (r *application) populateSchema(ctx context.Context, mg resource.Managed) (map[string]string, error) {
	log := r.log.WithValues("crName", mg.GetName())
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return nil, errors.New(errUnexpectedResource)
	}

	addressAllocationStrategy, err := r.registry.GetAddressAllocationStrategy(ctx, mg)
	if err != nil {
		return nil, err
	}

	niInfos, err := r.allocateNiIndexes(ctx, cr)
	if err != nil {
		return nil, err
	}

	for bdName := range cr.GetBridgeDomains() {
		// this provides all the ni info based on the initial allocation
		niInfo := niInfos[niinfo.GetBdName(bdName)]

		// get the epg, nodeitfce selectors belonging to this bridge domain
		// this validates and transforms the user input
		epgSelectors, nodeItfceSelectors, err := cr.GetBridgeEpgAndNodeItfceSelectors(bdName)
		if err != nil {
			return nil, err
		}
		log.Debug("bdName info", "epgSelectors", epgSelectors, "nodeItfceSelectors", nodeItfceSelectors)
		// get the nodes and interfaces based on the epg, nodeitfce selectors
		selectedNodeItfces, err := r.srlNddaHandler.GetSelectedNodeItfces(mg, epgSelectors, nodeItfceSelectors)
		if err != nil {
			return nil, err
		}
		log.Debug("bdName info", "selectedNodeItfces", selectedNodeItfces)
		for deviceName, itfces := range selectedNodeItfces {
			for _, itfceInfo := range itfces {
				r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)
			}
		}

		// if the tunnel mechanism in the bridge domin is vxlan add the vxlan interface
		if cr.GetBridgeDomainTunnel(bdName) == vpcv1alpha1.TunnelVxlan.String() {
			for deviceName := range selectedNodeItfces {
				itfceInfo := itfceinfo.NewItfceInfo(
					itfceinfo.WithItfceName("vxlan0"),
					itfceinfo.WithItfceKind(networkv1alpha1.E_InterfaceKind_VXLAN),
				)
				r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)
			}
		}
	}

	for rtName := range cr.GetRoutingTables() {
		// this provides all the ni info based on the initial allocation
		niInfo := niInfos[niinfo.GetRtName(rtName)]

		// get the epg, nodeitfce selectors belonging to this bridge domain
		epgSelectors, nodeItfceSelectors, err := cr.GetRoutingTableEpgAndNodeItfceSelectors(rtName)
		if err != nil {
			return nil, err
		}
		//log.Debug("routed epgSelectors", "epgSelectors", epgSelectors)
		//log.Debug("routed nodeItfceSelectors", "nodeItfceSelectors", nodeItfceSelectors)
		log.Debug("rtInfo info", "epgSelectors", epgSelectors, "nodeItfceSelectors", nodeItfceSelectors)
		// get the nodes and interfaces based on the epg, nodeitfce selectors
		selectedNodeItfces, err := r.srlNddaHandler.GetSelectedNodeItfces(mg, epgSelectors, nodeItfceSelectors)
		if err != nil {
			return nil, err
		}

		log.Debug("rtInfo info", "selectedNodeItfces", selectedNodeItfces)
		for deviceName, itfces := range selectedNodeItfces {
			for _, itfceInfo := range itfces {
				r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)
			}
		}

		log.Debug("vxlan info", "vxlan", cr.GetRoutingTableTunnel(rtName))

		// if the tunnel mechanism in the bridge domin is vxlan add the vxlan interface
		if cr.GetRoutingTableTunnel(rtName) == vpcv1alpha1.TunnelVxlan.String() {
			for deviceName := range selectedNodeItfces {
				itfceInfo := itfceinfo.NewItfceInfo(
					itfceinfo.WithItfceName("vxlan0"),
					itfceinfo.WithItfceKind(networkv1alpha1.E_InterfaceKind_VXLAN),
				)
				r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)
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
				// get the epg, nodeitfce selectors belonging to this bridge domain
				// this validates and transforms the user input
				epgSelectors, nodeItfceSelectors, err := cr.GetBridgeEpgAndNodeItfceSelectors(bdName)
				if err != nil {
					return nil, err
				}
				r.log.Debug("bdName info", "epgSelectors", epgSelectors, "nodeItfceSelectors", nodeItfceSelectors)
				// get the nodes and interfaces based on the epg, nodeitfce selectors
				selectedNodeItfces, err := r.srlNddaHandler.GetSelectedNodeItfces(mg, epgSelectors, nodeItfceSelectors)
				if err != nil {
					return nil, err
				}
				r.log.Debug("bdName info", "selectedNodeItfces", selectedNodeItfces)

				for deviceName := range selectedNodeItfces {
					// this provides all the ni info based on the initial allocation
					niInfo := niInfos[niinfo.GetBdName(bdName)]

					itfceInfo := itfceinfo.NewItfceInfo(
						itfceinfo.WithItfceName("irb0"),
						itfceinfo.WithItfceKind(networkv1alpha1.E_InterfaceKind_IRB),
					)
					r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)

					// add vxlan in bridged domain
					if cr.GetRoutingTableTunnel(rtName) == vpcv1alpha1.TunnelVxlan.String() {
						itfceInfo := itfceinfo.NewItfceInfo(
							itfceinfo.WithItfceName("vxlan0"),
							itfceinfo.WithItfceKind(networkv1alpha1.E_InterfaceKind_VXLAN),
						)
						r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)
					}

					itfceInfo.SetIpv4Prefixes(bd.GetIPv4Prefixes())
					itfceInfo.SetIpv6Prefixes(bd.GetIPv6Prefixes())

					niInfo = niInfos[niinfo.GetRtName(rtName)]

					r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)

					// add vxlan in rt table
					if cr.GetRoutingTableTunnel(rtName) == vpcv1alpha1.TunnelVxlan.String() {
						itfceInfo := itfceinfo.NewItfceInfo(
							itfceinfo.WithItfceName("vxlan0"),
							itfceinfo.WithItfceKind(networkv1alpha1.E_InterfaceKind_VXLAN),
						)
						r.PopulateSchema(ctx, mg, deviceName, itfceInfo, niInfo, addressAllocationStrategy)
					}
				}
			}
		}
	}

	return nil, nil
}
