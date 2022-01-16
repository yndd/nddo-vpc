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

package v1alpha1

import (
	"errors"
	"reflect"

	nddv1 "github.com/yndd/ndd-runtime/apis/common/v1"
	"github.com/yndd/ndd-runtime/pkg/resource"
	"github.com/yndd/ndd-runtime/pkg/utils"
	nddov1 "github.com/yndd/nddo-runtime/apis/common/v1"
	"github.com/yndd/nddo-runtime/pkg/odns"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ VpList = &VpcList{}

// +k8s:deepcopy-gen=false
type VpList interface {
	client.ObjectList

	GetVpcs() []Vp
}

func (x *VpcList) GetVpcs() []Vp {
	xs := make([]Vp, len(x.Items))
	for i, r := range x.Items {
		r := r // Pin range variable so we can take its address.
		xs[i] = &r
	}
	return xs
}

var _ Vp = &Vpc{}

// +k8s:deepcopy-gen=false
type Vp interface {
	resource.Object
	resource.Conditioned

	GetCondition(ct nddv1.ConditionKind) nddv1.Condition
	SetConditions(c ...nddv1.Condition)
	GetDeploymentPolicy() nddov1.DeploymentPolicy
	SetDeploymentPolicy(nddov1.DeploymentPolicy)
	GetOrganization() string
	GetDeployment() string
	GetAvailabilityZone() string
	GetVpcName() string
	GetAdminState() string
	GetDescription() string
	InitializeResource() error

	SetStatus(string)
	SetReason(string)
	GetStatus() string
	SetOrganization(string)
	SetDeployment(string)
	SetAvailabilityZone(s string)
	SetVpcName(s string)

	GetBridgeDomains() map[string]*VpcVpcBridgedDomains
	GetBridgeEpgAndNodeItfceSelectors(bd string) ([]*nddov1.EpgInfo, map[string]*nddov1.ItfceInfo, error)
	GetBridgeDomainProtocol(string) string
	GetBridgeDomainTunnel(string) string
	GetRoutingTables() map[string]*VpcVpcRoutingTables
	GetRoutingTableEpgAndNodeItfceSelectors(bd string) ([]*nddov1.EpgInfo, map[string]*nddov1.ItfceInfo, error)
	GetRoutingTableProtocol(string) string
	GetRoutingTableTunnel(string) string
	GetBridge2RoutingTable() map[string][]*VpcVpcRoutingTablesBridgeDomains
}

// GetCondition of this Network Node.
func (x *Vpc) GetCondition(ct nddv1.ConditionKind) nddv1.Condition {
	return x.Status.GetCondition(ct)
}

// SetConditions of the Network Node.
func (x *Vpc) SetConditions(c ...nddv1.Condition) {
	x.Status.SetConditions(c...)
}

func (x *Vpc) GetDeploymentPolicy() nddov1.DeploymentPolicy {
	return x.Spec.DeploymentPolicy
}

func (x *Vpc) SetDeploymentPolicy(c nddov1.DeploymentPolicy) {
	x.Spec.DeploymentPolicy = c
}

func (x *Vpc) GetOrganization() string {
	return odns.Name2OdnsResource(x.GetName()).GetOrganization()
}

func (x *Vpc) GetDeployment() string {
	return odns.Name2OdnsResource(x.GetName()).GetDeployment()
}

func (x *Vpc) GetAvailabilityZone() string {
	return odns.Name2OdnsResource(x.GetName()).GetAvailabilityZone()
}

func (x *Vpc) GetVpcName() string {
	return odns.Name2OdnsResource(x.GetName()).GetResourceName()
}

func (x *Vpc) GetAdminState() string {
	if reflect.ValueOf(x.Spec.Vpc.AdminState).IsZero() {
		return ""
	}
	return *x.Spec.Vpc.AdminState
}

func (x *Vpc) GetDescription() string {
	if reflect.ValueOf(x.Spec.Vpc.Description).IsZero() {
		return ""
	}
	return *x.Spec.Vpc.Description
}

func (x *Vpc) InitializeResource() error {
	if x.Status.Vpc != nil {
		// pool was already initialiazed
		// copy the spec, but not the state
		x.Status.Vpc.AdminState = x.Spec.Vpc.AdminState
		x.Status.Vpc.Description = x.Spec.Vpc.Description
		return nil
	}

	x.Status.Vpc = &NddoVpcVpc{
		AdminState:  x.Spec.Vpc.AdminState,
		Description: x.Spec.Vpc.Description,
		State: &NddoVpcVpcState{
			Status: utils.StringPtr(""),
			Reason: utils.StringPtr(""),
			Node:   make([]*NddoVpcVpcStateNode, 0),
			Link:   make([]*NddoVpcVpcStateLink, 0),
		},
	}
	return nil
}

func (x *Vpc) SetStatus(s string) {
	x.Status.Vpc.State.Status = &s
}

func (x *Vpc) SetReason(s string) {
	x.Status.Vpc.State.Reason = &s
}

func (x *Vpc) GetStatus() string {
	if x.Status.Vpc != nil && x.Status.Vpc.State != nil && x.Status.Vpc.State.Status != nil {
		return *x.Status.Vpc.State.Status
	}
	return "unknown"
}

func (x *Vpc) SetOrganization(s string) {
	x.Status.SetOrganization(s)
}

func (x *Vpc) SetDeployment(s string) {
	x.Status.SetDeployment(s)
}

func (x *Vpc) SetAvailabilityZone(s string) {
	x.Status.SetAvailabilityZone(s)
}

func (x *Vpc) SetVpcName(s string) {
	x.Status.VpcName = &s
}

func (x *Vpc) GetBridgeDomains() map[string]*VpcVpcBridgedDomains {
	bd := make(map[string]*VpcVpcBridgedDomains)
	if reflect.ValueOf(x.Spec.Vpc.BridgeDomains).IsZero() {
		return bd
	}
	for _, bridgeDomain := range x.Spec.Vpc.BridgeDomains {
		bd[*bridgeDomain.Name] = bridgeDomain
	}
	return bd
}

func (x *Vpc) GetRoutingTables() map[string]*VpcVpcRoutingTables {
	rt := make(map[string]*VpcVpcRoutingTables)
	if reflect.ValueOf(x.Spec.Vpc.RoutingTables).IsZero() {
		return rt
	}
	for _, routingTable := range x.Spec.Vpc.RoutingTables {
		rt[*routingTable.Name] = routingTable
	}
	return rt
}

func (x *Vpc) GetBridge2RoutingTable() map[string][]*VpcVpcRoutingTablesBridgeDomains {
	irb := make(map[string][]*VpcVpcRoutingTablesBridgeDomains)
	if reflect.ValueOf(x.Spec.Vpc.RoutingTables).IsZero() {
		return irb
	}

	for _, routingTable := range x.Spec.Vpc.RoutingTables {
		irb[*routingTable.Name] = routingTable.BridgeDomains

	}
	return irb
}

func (x *Vpc) GetBridgeEpgAndNodeItfceSelectors(bd string) ([]*nddov1.EpgInfo, map[string]*nddov1.ItfceInfo, error) {
	if bridgedomain, ok := x.GetBridgeDomains()[bd]; ok {
		return bridgedomain.InterfaceSelectors.GetEpgAndNodeItfceInfo()
	}
	return nil, nil, errors.New("routing table does not exist")
}

func (x *Vpc) GetBridgeDomainProtocol(bd string) string {
	if bridgedomain, ok := x.GetBridgeDomains()[bd]; ok {
		if reflect.ValueOf(bridgedomain.Protocol).IsZero() {
			if reflect.ValueOf(x.Spec.Vpc.Defaults.Protocol).IsZero() {
				return ""
			}
			return *x.Spec.Vpc.Defaults.Protocol
		}
		return *bridgedomain.Protocol
	}
	return ""
}

func (x *Vpc) GetBridgeDomainTunnel(bd string) string {
	if bridgedomain, ok := x.GetBridgeDomains()[bd]; ok {
		if reflect.ValueOf(bridgedomain.Tunnel).IsZero() {
			if reflect.ValueOf(x.Spec.Vpc.Defaults.Tunnel).IsZero() {
				return ""
			}
			return *x.Spec.Vpc.Defaults.Tunnel
		}
		return *bridgedomain.Tunnel
	}
	return ""
}

func (x *Vpc) GetRoutingTableEpgAndNodeItfceSelectors(rt string) ([]*nddov1.EpgInfo, map[string]*nddov1.ItfceInfo, error) {
	if routingTable, ok := x.GetRoutingTables()[rt]; ok {
		return routingTable.InterfaceSelectors.GetEpgAndNodeItfceInfo()
	}
	return nil, nil, errors.New("routing table does not exist")
}

func (x *Vpc) GetRoutingTableProtocol(rt string) string {
	if routingTable, ok := x.GetRoutingTables()[rt]; ok {
		if reflect.ValueOf(routingTable.Protocol).IsZero() {
			if reflect.ValueOf(x.Spec.Vpc.Defaults.Protocol).IsZero() {
				return ""
			}
			return *x.Spec.Vpc.Defaults.Protocol
		}
		return *routingTable.Protocol
	}
	return ""
}

func (x *Vpc) GetRoutingTableTunnel(rt string) string {
	if routingTable, ok := x.GetRoutingTables()[rt]; ok {
		if reflect.ValueOf(routingTable.Tunnel).IsZero() {
			if reflect.ValueOf(x.Spec.Vpc.Defaults.Tunnel).IsZero() {
				return ""
			}
			return *x.Spec.Vpc.Defaults.Tunnel
		}
		return *routingTable.Tunnel
	}
	return ""
}

type AddressingScheme string

const (
	AddressingSchemeDualStack AddressingScheme = "dual-stack"
	AddressingSchemeIpv4Only  AddressingScheme = "ipv4-only"
	AddressingSchemeIpv6Only  AddressingScheme = "ipv6-only"
)

func (s AddressingScheme) String() string {
	switch s {
	case AddressingSchemeDualStack:
		return "dual-stack"
	case AddressingSchemeIpv4Only:
		return "ipv4-only"
	case AddressingSchemeIpv6Only:
		return "ipv6-only"
	}
	return "unknown"
}

type Tunnel string

const (
	TunnelVxlan Tunnel = "vxlan"
	TunnelMpls  Tunnel = "mpls"
	TunnelSrv6  Tunnel = "srv6"
)

func (s Tunnel) String() string {
	switch s {
	case TunnelVxlan:
		return "vxlan"
	case TunnelMpls:
		return "mpls"
	case TunnelSrv6:
		return "srv6"
	}
	return "unknown"
}

type Protocol string

const (
	ProtocolEBGP        Protocol = "ebgp"
	ProtocolIBGP        Protocol = "ibgp"
	ProtocolISIS        Protocol = "isis"
	ProtocolOSPF        Protocol = "ospf"
	ProtocolEVPN        Protocol = "evpn"
	ProtocolIPVPN       Protocol = "ipvpn"
	ProtocolRouteTarget Protocol = "route-target"
)

func (s Protocol) String() string {
	switch s {
	case ProtocolEBGP:
		return "ebgp"
	case ProtocolIBGP:
		return "ibgp"
	case ProtocolISIS:
		return "isis"
	case ProtocolOSPF:
		return "ospf"
	case ProtocolEVPN:
		return "evpn"
	case ProtocolIPVPN:
		return "ipvpn"
	case ProtocolRouteTarget:
		return "route-target"
	}
	return "unknown"
}

func (x *VpcVpcRoutingTablesBridgeDomains) GetName() string {
	if reflect.ValueOf(x.Name).IsZero() {
		return ""
	}
	return *x.Name
}

func (x *VpcVpcRoutingTablesBridgeDomains) GetIPv4Prefixes() []*string {
	if reflect.ValueOf(x.Ipv4Prefixes).IsZero() {
		return make([]*string, 0)
	}
	return x.Ipv4Prefixes
}

func (x *VpcVpcRoutingTablesBridgeDomains) GetIPv6Prefixes() []*string {
	if reflect.ValueOf(x.Ipv6Prefixes).IsZero() {
		return make([]*string, 0)
	}
	return x.Ipv6Prefixes
}
