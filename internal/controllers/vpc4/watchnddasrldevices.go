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

	//ndddvrv1 "github.com/yndd/ndd-core/apis/dvr/v1"
	"github.com/yndd/ndd-runtime/pkg/logging"
	vpcv1alpha1 "github.com/yndd/nddo-vpc/apis/vpc/v1alpha1"
	"github.com/yndd/nddo-vpc/internal/speedyhandler"
	srlv1alpha1 "github.com/yndd/nddp-srl3/apis/srl3/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type adder interface {
	Add(item interface{})
}

type EnqueueRequestForAllNddaSrlDevices struct {
	client client.Client
	log    logging.Logger
	ctx    context.Context

	speedyHandler speedyhandler.Handler

	newVpcList func() vpcv1alpha1.VpList
}

// Create enqueues a request for all vpcs which pertains to the topology.
func (e *EnqueueRequestForAllNddaSrlDevices) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Create enqueues a request for all vpcs which pertains to the topology.
func (e *EnqueueRequestForAllNddaSrlDevices) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.ObjectOld, q)
	e.add(evt.ObjectNew, q)
}

// Create enqueues a request for all vpcs which pertains to the topology.
func (e *EnqueueRequestForAllNddaSrlDevices) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Create enqueues a request for all vpcs which pertains to the topology.
func (e *EnqueueRequestForAllNddaSrlDevices) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

func (e *EnqueueRequestForAllNddaSrlDevices) add(obj runtime.Object, queue adder) {
	dd, ok := obj.(*srlv1alpha1.Srl3Device)
	if !ok {
		return
	}
	log := e.log.WithValues("function", "watch ndda interface", "name", dd.GetName())
	log.Debug("vpc handleEvent")

	d := e.newVpcList()
	opts := []client.ListOption{
		client.InNamespace(dd.GetNamespace()),
	}
	if err := e.client.List(e.ctx, d, opts...); err != nil {
		return
	}

	for _, vpc := range d.GetVpcs() {

		if vpc.GetNamespace() == dd.GetNamespace() {

			crName := getCrName(vpc)
			e.speedyHandler.ResetSpeedy(crName)

			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: vpc.GetNamespace(),
				Name:      vpc.GetName()}})
		}
	}
}
