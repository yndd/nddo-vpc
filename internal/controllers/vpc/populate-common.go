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

	"github.com/yndd/nddo-runtime/pkg/resource"
	topov1alpha1 "github.com/yndd/nddr-topo-registry/apis/topo/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type nodeInfo struct {
	name     string // devicename
	kind     string // srl, sros, etc
	platform string // ixrd1, ixrd2, ixrd3, etc
	position string // spine or leaf
	nodeIdx  uint32 // index of the node in the system
}

func (r *application) gatherNodeInfo(ctx context.Context, mg resource.Managed) ([]*nodeInfo, error) {
	node := r.newTopoNodeList()
	opts := []client.ListOption{
		client.InNamespace(mg.GetNamespace()),
	}
	if err := r.client.List(ctx, node, opts...); err != nil {
		return nil, err
	}

	ninfo := make([]*nodeInfo, 0)
	for _, node := range node.GetNodes() {
		ninfo = append(ninfo, getNodeInfo(node))
	}
	return ninfo, nil
}

func getNodeInfo(node topov1alpha1.Tn) *nodeInfo {
	return &nodeInfo{
		name:     node.GetNodeName(),
		kind:     node.GetKindName(),
		platform: node.GetPlatform(),
		position: node.GetPosition(),
		nodeIdx:  node.GetNodeIndex(),
	}
}
