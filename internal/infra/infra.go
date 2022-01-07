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

package infra

import (
	"fmt"

	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/nddo-grpc/resource/resourcepb"
	"github.com/yndd/nddo-runtime/pkg/resource"
)

// InfraOption is used to configure the Infra.
type InfraOption func(*infra)

func WithInfraLogger(log logging.Logger) InfraOption {
	return func(r *infra) {
		r.log = log
	}
}

func WithInfraK8sClient(c resource.ClientApplicator) InfraOption {
	return func(r *infra) {
		r.client = c
	}
}

func WithInfraIpamClient(c resourcepb.ResourceClient) InfraOption {
	return func(r *infra) {
		r.ipamClient = c
	}
}

func WithInfraAsPoolClient(c resourcepb.ResourceClient) InfraOption {
	return func(r *infra) {
		r.aspoolClient = c
	}
}

func WithInfraNiRegisterClient(c resourcepb.ResourceClient) InfraOption {
	return func(r *infra) {
		r.niregisterClient = c
	}
}

func NewInfra(opts ...InfraOption) Infra {
	i := &infra{
		nodes: make(map[string]Node),
		links: make(map[string]Link),
		nis:   make(map[string]Ni),
	}

	for _, f := range opts {
		f(i)
	}

	return i
}

var _ Infra = &infra{}

type Infra interface {
	GetNodes() map[string]Node
	GetLinks() map[string]Link
	GetNis() map[string]Ni
	PrintNodes(string)
}

type infra struct {
	client           resource.ClientApplicator
	ipamClient       resourcepb.ResourceClient
	aspoolClient     resourcepb.ResourceClient
	niregisterClient resourcepb.ResourceClient
	log              logging.Logger

	nodes map[string]Node
	links map[string]Link
	nis   map[string]Ni
}

func (x *infra) GetNodes() map[string]Node {
	return x.nodes
}

func (x *infra) GetLinks() map[string]Link {
	return x.links
}

func (x *infra) GetNis() map[string]Ni {
	return x.nis
}

func (x *infra) GetLink(n string) Link {
	for _, link := range x.links {
		if link.GetName() == n {
			return link
		}
	}
	return nil
}

func (x *infra) PrintNodes(crname string) {
	fmt.Printf("infrastructure node information: %s\n", crname)
	for name, n := range x.GetNodes() {
		n.Print(name, 1)
	}
}
