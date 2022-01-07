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
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/ndd-runtime/pkg/meta"
	"github.com/yndd/ndd-runtime/pkg/utils"
	networkv1alpha1 "github.com/yndd/ndda-network/apis/network/v1alpha1"
	"github.com/yndd/nddo-grpc/resource/resourcepb"
	nddov1 "github.com/yndd/nddo-runtime/apis/common/v1"
	"github.com/yndd/nddo-runtime/pkg/odns"
	"github.com/yndd/nddo-runtime/pkg/resource"
	vpcv1alpha1 "github.com/yndd/nddo-vpc/apis/vpc/v1alpha1"
	ipamv1alpha1 "github.com/yndd/nddr-ipam-registry/apis/ipam/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	SubInterfacePrefix = "vpc"

	errCreateSubInterface = "cannot create SubInterface"
	errDeleteSubInterface = "cannot delete SubInterface"
	errGetSubInterface    = "cannot get SubInterface"
)

type SubInterfaceKind string

const (
	SubInterfaceKindBridged SubInterfaceKind = "bridged"
	SubInterfaceKindRouted  SubInterfaceKind = "routed"
)

func (s SubInterfaceKind) String() string {
	switch s {
	case SubInterfaceKindBridged:
		return "bridged"
	case SubInterfaceKindRouted:
		return "routed"
	}
	return "routed"
}

type TaggingKind string

const (
	TaggingKindUnTagged     TaggingKind = "untagged"
	TaggingKindSingleTagged TaggingKind = "singleTagged"
	TaggingKindDoubleTagged TaggingKind = "doubleTagged"
)

func (s TaggingKind) String() string {
	switch s {
	case TaggingKindUnTagged:
		return "untagged"
	case TaggingKindSingleTagged:
		return "singleTagged"
	case TaggingKindDoubleTagged:
		return "doubleTagged"
	}
	return "untagged"
}

// InfraOption is used to configure the Infra.
type SubInterfaceOption func(*subInterface)

func WithSubInterfaceLogger(log logging.Logger) SubInterfaceOption {
	return func(r *subInterface) {
		r.log = log
	}
}

func WithSubInterfaceK8sClient(c resource.ClientApplicator) SubInterfaceOption {
	return func(r *subInterface) {
		r.client = c
	}
}

func WithSubInterfaceIpamClient(c resourcepb.ResourceClient) SubInterfaceOption {
	return func(r *subInterface) {
		r.ipamClient = c
	}
}

func WithSubInterfaceAsPoolClient(c resourcepb.ResourceClient) SubInterfaceOption {
	return func(r *subInterface) {
		r.aspoolClient = c
	}
}

func WithSubInterfaceNiRegisterClient(c resourcepb.ResourceClient) SubInterfaceOption {
	return func(r *subInterface) {
		r.niregisterClient = c
	}
}

func NewSubInterface(itfce Interface, index string, opts ...SubInterfaceOption) SubInterface {
	i := &subInterface{
		itfce: itfce,
		index: &index,
		ipv4:  make(map[string]AddressInfo),
		ipv6:  make(map[string]AddressInfo),
	}

	for _, f := range opts {
		f(i)
	}

	return i
}

var _ SubInterface = &subInterface{}

type SubInterface interface {
	GetInterface() Interface
	GetNi() Ni
	SetNi(Ni)
	DeleteNi()
	GetIndex() string
	GetNeighbor() SubInterface
	GetTaggingKind() string
	GetKind() string
	GetOuterTag() uint32
	GetInnerTag() uint32
	GetEpgName() string
	GetItfceSelectorKind() nddov1.InterfaceSelectorKind
	GetItfceSelectorTags() []*nddov1.Tag
	GetAddressesInfo(af string) map[string]AddressInfo
	GetPrefixes(af string) []*string
	GetInterfaceSubInterfaceName() string
	GetBridgeDomainName() string
	SetIndex(string)
	SetNeighbor(SubInterface)
	SetTaggingKind(TaggingKind)
	SetKind(SubInterfaceKind)
	SetOuterTag(uint32)
	SetInnerTag(uint32)
	SetEpgName(string)
	SetItfceSelectorKind(nddov1.InterfaceSelectorKind)
	SetItfceSelectorTags([]*nddov1.Tag)
	SetBridgeDomainName(string)

	GetNddaSubInterface(ctx context.Context, cr vpcv1alpha1.Vp) (*string, error)
	CreateNddaSubInterface(ctx context.Context, cr vpcv1alpha1.Vp) error
	DeleteNddaSubInterface(ctx context.Context, cr vpcv1alpha1.Vp) error

	Print(string, int)
}

type subInterface struct {
	client           resource.ClientApplicator
	ipamClient       resourcepb.ResourceClient
	aspoolClient     resourcepb.ResourceClient
	niregisterClient resourcepb.ResourceClient
	log              logging.Logger

	itfce    Interface
	ni       Ni
	index    *string
	neighbor SubInterface
	tagging  TaggingKind
	kind     SubInterfaceKind
	outerTag *uint32
	innerTag *uint32
	ipv4     map[string]AddressInfo
	ipv6     map[string]AddressInfo
	epgName  *string

	itfceSelectorKind nddov1.InterfaceSelectorKind // the origin interface selector kind
	itfceSelectorTags []*nddov1.Tag
	bdName            *string
}

func (x *subInterface) GetInterface() Interface {
	return x.itfce
}

func (x *subInterface) GetNi() Ni {
	return x.ni
}

func (x *subInterface) SetNi(n Ni) {
	x.ni = n
}

func (x *subInterface) DeleteNi() {
	x.ni = nil
}

func (x *subInterface) GetIndex() string {
	if reflect.ValueOf(x.index).IsZero() {
		return ""
	}
	return *x.index
}

func (x *subInterface) GetNeighbor() SubInterface {
	return x.neighbor
}

func (x *subInterface) GetTaggingKind() string {
	return string(x.tagging)
}

func (x *subInterface) GetKind() string {
	return string(x.kind)
}

func (x *subInterface) GetOuterTag() uint32 {
	if reflect.ValueOf(x.outerTag).IsZero() {
		return 0
	}
	return *x.outerTag
}

func (x *subInterface) GetInnerTag() uint32 {
	if reflect.ValueOf(x.innerTag).IsZero() {
		return 0
	}
	return *x.innerTag
}

func (x *subInterface) GetEpgName() string {
	if reflect.ValueOf(x.epgName).IsZero() {
		return ""
	}
	return *x.epgName
}

func (x *subInterface) GetBridgeDomainName() string {
	if reflect.ValueOf(x.bdName).IsZero() {
		return ""
	}
	return *x.bdName
}

func (x *subInterface) GetItfceSelectorKind() nddov1.InterfaceSelectorKind {
	return x.itfceSelectorKind
}

func (x *subInterface) GetItfceSelectorTags() []*nddov1.Tag {
	return x.itfceSelectorTags
}

func (x *subInterface) SetInterface(i Interface) {
	x.itfce = i
}

func (x *subInterface) SetIndex(n string) {
	x.index = &n
}

func (x *subInterface) SetNeighbor(n SubInterface) {
	x.neighbor = n
}

func (x *subInterface) SetTaggingKind(n TaggingKind) {
	x.tagging = n
}

func (x *subInterface) SetKind(n SubInterfaceKind) {
	x.kind = n
}

func (x *subInterface) SetInnerTag(t uint32) {
	x.innerTag = &t
}

func (x *subInterface) SetOuterTag(t uint32) {
	x.outerTag = &t
}

func (x *subInterface) SetEpgName(s string) {
	x.epgName = &s
}

func (x *subInterface) SetBridgeDomainName(s string) {
	x.bdName = &s
}

func (x *subInterface) SetItfceSelectorKind(s nddov1.InterfaceSelectorKind) {
	x.itfceSelectorKind = s
}

func (x *subInterface) SetItfceSelectorTags(s []*nddov1.Tag) {
	x.itfceSelectorTags = s
}

func (x *subInterface) GetAddressesInfo(af string) map[string]AddressInfo {
	switch af {
	case string(ipamv1alpha1.AddressFamilyIpv4):
		return x.ipv4
	case string(ipamv1alpha1.AddressFamilyIpv6):
		return x.ipv6
	}
	return nil
}

func (x *subInterface) GetPrefixes(af string) []*string {
	prefixes := make([]*string, 0)
	switch af {
	case string(ipamv1alpha1.AddressFamilyIpv4):
		for prefix := range x.ipv4 {
			prefixes = append(prefixes, &prefix)
		}
	case string(ipamv1alpha1.AddressFamilyIpv6):
		for prefix := range x.ipv6 {
			prefixes = append(prefixes, &prefix)
		}
	}
	return prefixes
}

func (x *subInterface) GetInterfaceSubInterfaceName() string {
	return strings.Join([]string{x.GetInterface().GetName(), x.GetIndex()}, ".")
}

func (x *subInterface) GetNddaSubInterface(ctx context.Context, cr vpcv1alpha1.Vp) (*string, error) {
	c := x.buildNddaSubInterface(cr)
	if err := x.client.Get(ctx, types.NamespacedName{
		Namespace: cr.GetNamespace(), Name: c.GetName()}, c); err != nil {
		return nil, errors.Wrap(err, errGetSubInterface)
	}
	return utils.StringPtr(c.GetName()), nil
}

func (x *subInterface) CreateNddaSubInterface(ctx context.Context, cr vpcv1alpha1.Vp) error {
	c := x.buildNddaSubInterface(cr)
	if err := x.client.Apply(ctx, c); err != nil {
		return errors.Wrap(err, errDeleteSubInterface)
	}
	return nil

}

func (x *subInterface) DeleteNddaSubInterface(ctx context.Context, cr vpcv1alpha1.Vp) error {
	c := x.buildNddaSubInterface(cr)
	if err := x.client.Delete(ctx, c); err != nil {
		return errors.Wrap(err, errDeleteSubInterface)
	}
	return nil
}

func (x *subInterface) buildNddaSubInterface(cr vpcv1alpha1.Vp) *networkv1alpha1.SubInterface {

	index := strings.ReplaceAll(x.GetIndex(), "/", "-")
	itfceName := strings.ReplaceAll(x.GetInterface().GetName(), "/", "-")

	//vpcName := cr.GetVpcName()

	resourceName := odns.GetOdnsResourceName(cr.GetName(), strings.ToLower(vpcv1alpha1.VpcKindKind),
		[]string{x.GetInterface().GetNode().GetName(), itfceName, index, x.GetKind()})

	objMeta := metav1.ObjectMeta{
		//Name:      strings.Join([]string{vpcName, x.GetInterface().GetNode().GetName(), itfceName, index, x.GetKind()}, "."),
		Name:      resourceName,
		Namespace: cr.GetNamespace(),
		Labels: map[string]string{
			networkv1alpha1.LabelSubInterfaceKindKey: x.GetKind(),
		},
		OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(cr, vpcv1alpha1.VpcGroupVersionKind))},
	}

	switch x.GetKind() {
	case SubInterfaceKindBridged.String():
		return &networkv1alpha1.SubInterface{
			ObjectMeta: objMeta,
			Spec: networkv1alpha1.SubInterfaceSpec{
				//TopologyName:  utils.StringPtr(cr.GetTopologyName()),
				NodeName:      utils.StringPtr(x.GetInterface().GetNode().GetName()),
				InterfaceName: utils.StringPtr(x.GetInterface().GetName()),
				//EndpointGroup: utils.StringPtr(cr.GetName()),
				SubInterface: &networkv1alpha1.NetworkSubInterface{
					Index:    utils.StringPtr(x.GetIndex()),
					Kind:     utils.StringPtr(x.GetKind()),
					Tagging:  utils.StringPtr(x.GetTaggingKind()),
					OuterTag: utils.Uint32Ptr(x.GetOuterTag()),
					InnerTag: utils.Uint32Ptr(x.GetInnerTag()),
				},
			},
		}
	default:
		// routed
		ipv4Prefixes := make([]*string, 0)
		for _, ai := range x.GetAddressesInfo(ipamv1alpha1.AddressFamilyIpv4.String()) {
			ipv4Prefixes = append(ipv4Prefixes, utils.StringPtr(ai.GetPrefix()))
		}
		ipv6Prefixes := make([]*string, 0)
		for _, ai := range x.GetAddressesInfo(ipamv1alpha1.AddressFamilyIpv6.String()) {
			ipv6Prefixes = append(ipv6Prefixes, utils.StringPtr(ai.GetPrefix()))
		}

		return &networkv1alpha1.SubInterface{
			ObjectMeta: objMeta,
			Spec: networkv1alpha1.SubInterfaceSpec{
				//TopologyName:  utils.StringPtr(cr.GetTopologyName()),
				NodeName:      utils.StringPtr(x.GetInterface().GetNode().GetName()),
				InterfaceName: utils.StringPtr(x.GetInterface().GetName()),
				//EndpointGroup: utils.StringPtr(cr.GetName()),
				SubInterface: &networkv1alpha1.NetworkSubInterface{
					Index:    utils.StringPtr(x.GetIndex()),
					Kind:     utils.StringPtr(x.GetKind()),
					Tagging:  utils.StringPtr(x.GetTaggingKind()),
					OuterTag: utils.Uint32Ptr(x.GetOuterTag()),
					InnerTag: utils.Uint32Ptr(x.GetInnerTag()),
					Ipv4:     ipv4Prefixes,
					Ipv6:     ipv6Prefixes,
				},
			},
		}
	}
}

func (x *subInterface) Print(subItfceName string, n int) {
	fmt.Printf("%s SubInterface: %s Kind: %s Tagging: %s InterfaceSelecterKind: %s BdName: %s\n",
		strings.Repeat(" ", n), subItfceName, x.GetKind(), x.GetTaggingKind(), x.GetItfceSelectorKind(), x.GetBridgeDomainName())
	n++
	fmt.Printf("%s Local Addressing Info\n", strings.Repeat(" ", n))
	for prefix, i := range x.ipv4 {
		i.Print(string(ipamv1alpha1.AddressFamilyIpv4), prefix, n)
	}
	for prefix, i := range x.ipv6 {
		i.Print(string(ipamv1alpha1.AddressFamilyIpv6), prefix, n)
	}
	if x.neighbor != nil {
		fmt.Printf("%s Neighbor Addressing Info\n", strings.Repeat(" ", n))
		for prefix, i := range x.neighbor.GetAddressesInfo(string(ipamv1alpha1.AddressFamilyIpv4)) {
			i.Print(string(ipamv1alpha1.AddressFamilyIpv4), prefix, n)
		}
		for prefix, i := range x.neighbor.GetAddressesInfo(string(ipamv1alpha1.AddressFamilyIpv6)) {
			i.Print(string(ipamv1alpha1.AddressFamilyIpv6), prefix, n)
		}
	}
}
