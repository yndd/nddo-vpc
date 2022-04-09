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
	"strings"
	"time"

	"github.com/yndd/ndd-runtime/pkg/event"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/nddo-runtime/pkg/intent"

	//networkv1alpha1 "github.com/yndd/ndda-network/apis/network/v1alpha1"

	nddv1 "github.com/yndd/ndd-runtime/apis/common/v1"
	nddpresource "github.com/yndd/ndd-runtime/pkg/resource"
	"github.com/yndd/ndda-network/pkg/abstraction"
	"github.com/yndd/ndda-network/pkg/ndda/niinfo"
	"github.com/yndd/nddo-runtime/pkg/reconciler/managed"
	"github.com/yndd/nddo-runtime/pkg/resource"
	vpcv1alpha1 "github.com/yndd/nddo-vpc/apis/vpc/v1alpha1"
	"github.com/yndd/nddo-vpc/internal/nihandler"
	"github.com/yndd/nddo-vpc/internal/shared"
	"github.com/yndd/nddo-vpc/internal/speedyhandler"
	srlv1alpha1 "github.com/yndd/nddp-srl3/apis/srl3/v1alpha1"
	"github.com/yndd/nddr-org-registry/pkg/registry"
	topov1alpha1 "github.com/yndd/nddr-topo-registry/apis/topo/v1alpha1"
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
	tnlfn := func() topov1alpha1.TnList { return &topov1alpha1.TopologyNodeList{} }

	shandler := speedyhandler.New()

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(vpcv1alpha1.VpcGroupVersionKind),
		managed.WithLogger(nddcopts.Logger.WithValues("controller", name)),
		managed.WithApplication(&application{
			client: resource.ClientApplicator{
				Client:     mgr.GetClient(),
				Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
			},
			log:             nddcopts.Logger.WithValues("applogic", name),
			newVpc:          vpcfn,
			newVpcList:      vpclfn,
			newTopoNodeList: tnlfn,
			registry:        nddcopts.Registry,
			intents:         make(map[string]*intent.Compositeintent),
			abstractions:    make(map[string]*abstraction.Compositeabstraction),
			/*
				srlNddaHandler: srlndda.New(
					srlndda.WithClient(resource.ClientApplicator{
						Client:     mgr.GetClient(),
						Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
					}),
					srlndda.WithLogger(nddcopts.Logger.WithValues("nddasrlhandler", name)),
				),
			*/
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

	nddpSrlDeviceHandler := &EnqueueRequestForAllNddpSrlDevices{
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
		Watches(&source.Kind{Type: &srlv1alpha1.Srl3Device{}}, nddpSrlDeviceHandler).
		WithEventFilter(resource.IgnoreUpdateWithoutGenerationChangePredicate()).
		Complete(r)

}

type application struct {
	client resource.ClientApplicator
	log    logging.Logger

	newVpc          func() vpcv1alpha1.Vp
	newVpcList      func() vpcv1alpha1.VpList
	newTopoNode     func() topov1alpha1.Tn
	newTopoNodeList func() topov1alpha1.TnList

	registry registry.Registry
	//srlNddaHandler srlndda.Handler
	speedyHandler speedyhandler.Handler
	niHandler     nihandler.Handler
	intents       map[string]*intent.Compositeintent
	abstractions  map[string]*abstraction.Compositeabstraction
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
	crName := getCrName(mg)
	log := r.log.WithValues("crName", crName)
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return nil, errors.New(errUnexpectedResource)
	}
	log.Debug("Update ...")

	r.speedyHandler.Init(crName)

	//return r.handleAppLogic(ctx, cr)
	info, err := r.populateSchema(ctx, mg)
	if err != nil {
		return info, err
	}

	// list all resources which are currently in the system
	resources, err := r.intents[crName].List(ctx, mg, map[string]map[string]nddpresource.Managed{})
	if err != nil {
		log.Debug("intent list failed", "error", err)
		return nil, err
	}

	// validate if the resources listed before still exists -> returns a map with resource that should be deleted
	resources, err = r.intents[crName].Validate(ctx, mg, resources)
	if err != nil {
		log.Debug("intent validate failed", "error", err)
		return nil, err
	}

	// delete the resources which should no longer be present
	if err := r.intents[crName].Delete(ctx, mg, resources); err != nil {
		log.Debug("intent delete failed", "error", err)
		return nil, err
	}

	labels := make(map[string]string)
	if err := r.intents[crName].Deploy(ctx, mg, labels); err != nil {
		log.Debug("intent deploy failed", "error", err)
		return nil, err
	}

	// list all resources which are currently in the system
	resources, err = r.intents[crName].List(ctx, mg, map[string]map[string]nddpresource.Managed{})
	if err != nil {
		log.Debug("intent list failed", "error", err)
		return nil, err
	}
	for nddpKind, nddpManaged := range resources {
		for nddpName, nddpm := range nddpManaged {
			log.Debug("nddp status", "kind", nddpKind, "name", nddpName, "Status", nddpm.GetCondition(nddv1.ConditionKindReady))
		}
	}

	cr.SetOrganization(cr.GetOrganization())
	cr.SetDeployment(cr.GetDeployment())
	cr.SetAvailabilityZone(cr.GetAvailabilityZone())

	return nil, nil
}

func (r *application) FinalUpdate(ctx context.Context, mg resource.Managed) {
	// the reconciler always starts from scratch without history, hence we delete the intent data from the map
	delete(r.intents, getCrName(mg))
	delete(r.abstractions, getCrName(mg))
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
	_, err := r.populateSchema(ctx, mg)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *application) FinalDelete(ctx context.Context, mg resource.Managed) {
	crName := getCrName(mg)
	r.intents[crName].Destroy(ctx, mg, map[string]string{})
	r.speedyHandler.Delete(crName)
}

func (r *application) populateSchema(ctx context.Context, mg resource.Managed) (map[string]string, error) {
	log := r.log.WithValues("crName", mg.GetName())
	cr, ok := mg.(*vpcv1alpha1.Vpc)
	if !ok {
		return nil, errors.New(errUnexpectedResource)
	}

	crName := getCrName(mg)
	r.intents[crName] = intent.New(r.client, crName)
	//ci := r.intents[crName]
	r.abstractions[crName] = abstraction.New(r.client, crName)

	addressAllocationStrategy, err := r.registry.GetAddressAllocationStrategy(ctx, mg)
	if err != nil {
		return nil, err
	}

	niInfos, err := r.allocateNiIndexes(ctx, cr)
	if err != nil {
		return nil, err
	}

	// get all the nodes in the topology
	nodeInfo, err := r.gatherNodeInfo(ctx, mg)
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

		for _, n := range nodeInfo {
			switch n.kind {
			case "srl":
				if err := r.srlPopulateBridgeDomain(ctx, mg, bdName, n, niInfo, epgSelectors, nodeItfceSelectors, addressAllocationStrategy); err != nil {
					return nil, err
				}
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

		for _, n := range nodeInfo {
			switch n.kind {
			case "srl":
				if err := r.srlPopulateRouteTable(ctx, mg, rtName, n, niInfo, epgSelectors, nodeItfceSelectors, addressAllocationStrategy); err != nil {
					return nil, err
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
				// get the epg, nodeitfce selectors belonging to this bridge domain
				// this validates and transforms the user input
				epgSelectors, nodeItfceSelectors, err := cr.GetBridgeEpgAndNodeItfceSelectors(bdName)
				if err != nil {
					return nil, err
				}
				r.log.Debug("bdName info", "epgSelectors", epgSelectors, "nodeItfceSelectors", nodeItfceSelectors)

				for _, n := range nodeInfo {
					switch n.kind {
					case "srl":
						if err := r.srlPopulateRouteIrb(ctx, mg, bdName, rtName, n, niInfos, epgSelectors, nodeItfceSelectors, addressAllocationStrategy, bd); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}

	return nil, nil
}
