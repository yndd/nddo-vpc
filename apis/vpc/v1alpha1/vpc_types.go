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
	"reflect"

	nddov1 "github.com/yndd/nddo-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Vpc struct
type VpcVpc struct {
	// +kubebuilder:validation:Enum=`disable`;`enable`
	// +kubebuilder:default:="enable"
	AdminState *string `json:"admin-state,omitempty"`
	// kubebuilder:validation:MinLength=1
	// kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern="[A-Za-z0-9 !@#$^&()|+=`~.,'/_:;?-]*"
	Description   *string                 `json:"description,omitempty"`
	BridgeDomains []*VpcVpcBridgedDomains `json:"bridge-domains,omitempty"`
	RoutingTables []*VpcVpcRoutingTables  `json:"routing-tables,omitempty"`
	Defaults      VpcVpcDefaults          `json:"defaults,omitempty"`
}

type VpcVpcDefaults struct {
	// +kubebuilder:validation:Enum=`vxlan`;`mpls`;`srv6`
	// +kubebuilder:default:="vxlan"
	Tunnel *string `json:"tunnel,omitempty"`
	// +kubebuilder:validation:Enum=`evpn`;`ipvpn`
	// +kubebuilder:default:="evpn"
	Protocol *string `json:"protocol,omitempty"`
}

type VpcVpcBridgedDomains struct {
	Name *string `json:"name"`
	// +kubebuilder:validation:Enum=`vxlan`;`mpls`;`srv6`
	// +kubebuilder:default:="vxlan"
	Tunnel *string `json:"tunnel,omitempty"`
	// +kubebuilder:validation:Enum=`evpn`;`ipvpn`
	// +kubebuilder:default:="evpn"
	Protocol                  *string `json:"protocol,omitempty"`
	nddov1.InterfaceSelectors `json:",inline"`
}

type VpcVpcRoutingTables struct {
	Name          *string                             `json:"name"`
	BridgeDomains []*VpcVpcRoutingTablesBridgeDomains `json:"bridge-domains,omitempty"`
	//InterfaceSelectors []*nddov1.InterfaceSelector         `json:"interface-selectors,omitempty"`
	// +kubebuilder:validation:Enum=`vxlan`;`mpls`;`srv6`
	// +kubebuilder:default:="vxlan"
	Tunnel *string `json:"tunnel,omitempty"`
	// +kubebuilder:validation:Enum=`evpn`;`ipvpn`
	// +kubebuilder:default:="evpn"
	Protocol                  *string `json:"protocol,omitempty"`
	nddov1.InterfaceSelectors `json:",inline"`
}

type VpcVpcRoutingTablesBridgeDomains struct {
	Name         *string   `json:"name"`
	Ipv4Prefixes []*string `json:"ipv4-prefixes,omitempty"`
	Ipv6Prefixes []*string `json:"ipv6-prefixes,omitempty"`
}

// A VpcSpec defines the desired state of a Vpc.
type VpcSpec struct {
	//nddov1.OdaInfo `json:",inline"`
	nddov1.ResourceSpec `json:",inline"`
	Vpc                 *VpcVpc `json:"vpc,omitempty"`
}

// A VpcStatus represents the observed state of a VpcSpec.
type VpcStatus struct {
	nddov1.ResourceStatus `json:",inline"`
	VpcName               *string     `json:"vpc-name,omitempty"`
	Vpc                   *NddoVpcVpc `json:"Vpc,omitempty"`
}

// +kubebuilder:object:root=true

// Vpc is the Schema for the Vpc API
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNC",type="string",JSONPath=".status.conditions[?(@.kind=='Synced')].status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.conditions[?(@.kind=='Ready')].status"
// +kubebuilder:printcolumn:name="ORG",type="string",JSONPath=".status.oda[?(@.key=='organization')].value"
// +kubebuilder:printcolumn:name="DEP",type="string",JSONPath=".status.oda[?(@.key=='deployment')].value"
// +kubebuilder:printcolumn:name="AZ",type="string",JSONPath=".status.oda[?(@.key=='availability-zone')].value"
// +kubebuilder:printcolumn:name="VPC",type="string",JSONPath=".status.vpc-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories={ndd,nddo}
type Vpc struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VpcSpec   `json:"spec,omitempty"`
	Status VpcStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VpcList contains a list of Vpcs
type VpcList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Vpc `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Vpc{}, &VpcList{})
}

// Vpc type metadata.
var (
	VpcKindKind         = reflect.TypeOf(Vpc{}).Name()
	VpcGroupKind        = schema.GroupKind{Group: Group, Kind: VpcKindKind}.String()
	VpcKindAPIVersion   = VpcKindKind + "." + GroupVersion.String()
	VpcGroupVersionKind = GroupVersion.WithKind(VpcKindKind)
)
