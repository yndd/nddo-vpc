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
	"strings"
	"time"

	"github.com/yndd/ndd-runtime/pkg/event"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/ndd-runtime/pkg/utils"
	networkv1alpha1 "github.com/yndd/ndda-network/apis/network/v1alpha1"
	"github.com/yndd/nddo-runtime/pkg/nddo"
	"github.com/yndd/nddo-runtime/pkg/reconciler/managed"
	"github.com/yndd/nddo-runtime/pkg/resource"
	vpcv1alpha1 "github.com/yndd/nddo-vpc/apis/vpc/v1alpha1"
	"github.com/yndd/nddo-vpc/internal/schemahandler"
	"github.com/yndd/nddo-vpc/internal/shared"
	"github.com/yndd/nddo-vpc/pkg/schema"
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
			nddoHandler: nddo.New(
				nddo.WithClient(resource.ClientApplicator{
					Client:     mgr.GetClient(),
					Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
				}),
				nddo.WithLogger(nddcopts.Logger.WithValues("nddohandler", name)),
			),
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

	nddoHandler nddo.Handler
	handler     schemahandler.Handler
	registry    registry.Registry
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
	_, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return nil, errors.New(errUnexpectedResource)
	}

	return nil, r.populateSchema(ctx, mg)

	//return r.handleAppLogic(ctx, cr)
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

/*
func (r *application) handleAppLogic(ctx context.Context, cr vpcv1alpha1.Vp) (map[string]string, error) {

	// r.allocateResources(ctx, cr)

	// r.validateDelta(ctx, cr)

	// r.applyIntent(ctx, cr)

	return nil, nil
}
*/

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
		selectedNodeItfces, err := r.nddoHandler.GetSelectedNodeItfces(mg, epgSelectors, nodeItfceSelectors)
		if err != nil {
			return err
		}
		for deviceName, itfces := range selectedNodeItfces {
			for _, itfceInfo := range itfces {
				niInfo := &schema.DeviceNetworkInstanceData{
					Name:  utils.StringPtr(bdName),
					Index: utils.Uint32Ptr(0), // to do allocation
					Kind:  utils.StringPtr("bridged"),
				}
				s.PopulateSchema(deviceName, itfceInfo, niInfo, addressAllocationStrategy)
			}
		}

		// if the tunnel mechanism in the bridge domin is vxlan add the vxlan interface
		if cr.GetBridgeDomainTunnel(bdName) == vpcv1alpha1.TunnelVxlan.String() {
			selectedNodeItfces, err := r.nddoHandler.GetSelectedNodeItfcesVxlan(mg, s, bdName)
			if err != nil {
				return err
			}
			for deviceName, itfces := range selectedNodeItfces {
				for _, itfceInfo := range itfces {
					niInfo := &schema.DeviceNetworkInstanceData{
						Name:  utils.StringPtr(bdName),
						Index: utils.Uint32Ptr(0), // to do allocation
						Kind:  utils.StringPtr("bridged"),
					}
					s.PopulateSchema(deviceName, itfceInfo, niInfo, addressAllocationStrategy)
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
		selectedNodeItfces, err := r.nddoHandler.GetSelectedNodeItfces(mg, epgSelectors, nodeItfceSelectors)
		if err != nil {
			return err
		}
		for deviceName, itfces := range selectedNodeItfces {
			for _, itfceInfo := range itfces {
				niInfo := &schema.DeviceNetworkInstanceData{
					Name:  utils.StringPtr(rtName),
					Index: utils.Uint32Ptr(0), // to do allocation
					Kind:  utils.StringPtr("routed"),
				}
				s.PopulateSchema(deviceName, itfceInfo, niInfo, addressAllocationStrategy)
			}
		}
		// if the tunnel mechanism in the bridge domin is vxlan add the vxlan interface
		if cr.GetBridgeDomainTunnel(rtName) == vpcv1alpha1.TunnelVxlan.String() {
			selectedNodeItfces, err := r.nddoHandler.GetSelectedNodeItfcesVxlan(mg, s, rtName)
			if err != nil {
				return err
			}
			for deviceName, itfces := range selectedNodeItfces {
				for _, itfceInfo := range itfces {

					niInfo := &schema.DeviceNetworkInstanceData{
						Name:  utils.StringPtr(rtName),
						Index: utils.Uint32Ptr(0), // to do allocation
						Kind:  utils.StringPtr("routed"),
					}
					s.PopulateSchema(deviceName, itfceInfo, niInfo, addressAllocationStrategy)
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
				selectedNodeItfces, err := r.nddoHandler.GetSelectedNodeItfcesIrb(mg, s, rtName)
				if err != nil {
					return err
				}
				for deviceName, itfces := range selectedNodeItfces {
					for _, itfceInfo := range itfces {

						niInfo := &schema.DeviceNetworkInstanceData{
							Name:  utils.StringPtr(rtName),
							Index: utils.Uint32Ptr(0), // to do allocation
							Kind:  utils.StringPtr("bridged"),
						}
						s.PopulateSchema(deviceName, itfceInfo, niInfo, addressAllocationStrategy)

						// the ipv4/ipv6 prefix info comes from the bd interface
						itfceInfo.SetIpv4Prefixes(bd.GetIPv4Prefixes())
						itfceInfo.SetIpv6Prefixes(bd.GetIPv6Prefixes())
						niInfo = &schema.DeviceNetworkInstanceData{
							Name:  utils.StringPtr(rtName),
							Index: utils.Uint32Ptr(0), // to do allocation
							Kind:  utils.StringPtr("routed"),
						}
						s.PopulateSchema(deviceName, itfceInfo, niInfo, addressAllocationStrategy)
					}
				}
			}
		}
	}

	return nil
}
