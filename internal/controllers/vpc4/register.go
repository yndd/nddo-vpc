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

	"github.com/yndd/ndd-runtime/pkg/utils"
	"github.com/yndd/ndda-network/pkg/ndda/niinfo"
	vpcv1alpha1 "github.com/yndd/nddo-vpc/apis/vpc/v1alpha1"
)

const (
	niRegistry = "nokia-default"
)

func (r *application) allocateNiIndexes(ctx context.Context, cr *vpcv1alpha1.Vpc) (map[string]*niinfo.NiInfo, error) {
	nis := make(map[string]*niinfo.NiInfo)
	// create the registers for the ni Allocations
	for bdName := range cr.GetBridgeDomains() {
		niBdName := niinfo.GetBdName(bdName)
		nis[niBdName] = &niinfo.NiInfo{
			Name:     utils.StringPtr(niBdName),
			Registry: utils.StringPtr(niRegistry),
		}
		if err := r.niHandler.CreateNiRegister(ctx, cr, nis[niBdName]); err != nil {
			return nil, err
		}
	}
	for rtName := range cr.GetRoutingTables() {
		niRtName := niinfo.GetRtName(rtName)
		nis[niRtName] = &niinfo.NiInfo{
			Name:     utils.StringPtr(niRtName),
			Registry: utils.StringPtr(niRegistry),
		}
		if err := r.niHandler.CreateNiRegister(ctx, cr, nis[niRtName]); err != nil {
			return nil, err
		}
	}
	for bdName := range cr.GetBridgeDomains() {
		niBdName := niinfo.GetBdName(bdName)
		niIndex, err := r.niHandler.GetNiRegister(ctx, cr, nis[niBdName])
		if err != nil {
			return nil, err
		}
		nis[niBdName].Index = niIndex
	}
	for rtName := range cr.GetRoutingTables() {
		niRtName := niinfo.GetRtName(rtName)
		niIndex, err := r.niHandler.GetNiRegister(ctx, cr, nis[niRtName])
		if err != nil {
			return nil, err
		}
		nis[niRtName].Index = niIndex
	}
	return nis, nil
}
