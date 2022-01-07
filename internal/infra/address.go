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
	AddressPrefix = "vpc"

	errCreatePrefix = "cannot create Prefix"
	errDeletePrefix = "cannot delete Prefix"
	errGetPrefix    = "cannot get Prefix"
)

// AddressOption is used to configure the Infra.
type AddressOption func(*addressInfo)

func WithAddressLogger(log logging.Logger) AddressOption {
	return func(r *addressInfo) {
		r.log = log
	}
}

func WithAddressK8sClient(c resource.ClientApplicator) AddressOption {
	return func(r *addressInfo) {
		r.client = c
	}
}

func WithAddressIpamClient(c resourcepb.ResourceClient) AddressOption {
	return func(r *addressInfo) {
		r.ipamClient = c
	}
}

func WithAddressAsPoolClient(c resourcepb.ResourceClient) AddressOption {
	return func(r *addressInfo) {
		r.aspoolClient = c
	}
}

func WithAddressNiRegisterClient(c resourcepb.ResourceClient) AddressOption {
	return func(r *addressInfo) {
		r.niregisterClient = c
	}
}

func NewAddressInfo(subitfce SubInterface, prefix string, opts ...AddressOption) AddressInfo {
	i := &addressInfo{
		prefix:   &prefix,
		subitfce: subitfce,
	}

	for _, f := range opts {
		f(i)
	}

	return i
}

var _ AddressInfo = &addressInfo{}

type AddressInfo interface {
	GetSubInterface() SubInterface
	GetPrefix() string
	GetCidr() string
	GetAddress() string
	GetPrefixLength() uint32
	SetCidr(string)
	SetPrefix(string)
	SetAddress(string)
	SetPrefixLength(uint32)

	GetIpamPrefix(ctx context.Context, cr vpcv1alpha1.Vp, ipamOptions *IpamOptions) (*string, error)
	CreateIpamPrefix(ctx context.Context, cr vpcv1alpha1.Vp, ipamOptions *IpamOptions) error
	DeleteIpamPrefix(ctx context.Context, cr vpcv1alpha1.Vp, ipamOptions *IpamOptions) error

	GrpcAllocateEndpointIP(ctx context.Context, cr vpcv1alpha1.Vp, ipamOptions *IpamOptions) (*string, error)
	GrpcDeAllocateEndpointIP(ctx context.Context, cr vpcv1alpha1.Vp, ipamOptions *IpamOptions) error

	Print(string, string, int)
}

type addressInfo struct {
	client           resource.ClientApplicator
	ipamClient       resourcepb.ResourceClient
	aspoolClient     resourcepb.ResourceClient
	niregisterClient resourcepb.ResourceClient
	log              logging.Logger

	subitfce     SubInterface
	cidr         *string
	prefix       *string
	address      *string
	prefixLength *uint32
}

func (x *addressInfo) GetSubInterface() SubInterface {
	return x.subitfce
}

func (x *addressInfo) GetPrefix() string {
	if reflect.ValueOf(x.prefix).IsZero() {
		return ""
	}
	return *x.prefix
}

func (x *addressInfo) GetCidr() string {
	if reflect.ValueOf(x.cidr).IsZero() {
		return ""
	}
	return *x.cidr
}

func (x *addressInfo) GetAddress() string {
	if reflect.ValueOf(x.address).IsZero() {
		return ""
	}
	return *x.address
}

func (x *addressInfo) GetPrefixLength() uint32 {
	if reflect.ValueOf(x.prefixLength).IsZero() {
		return 0
	}
	return *x.prefixLength
}

func (x *addressInfo) SetPrefix(s string) {
	x.prefix = &s
}

func (x *addressInfo) SetCidr(s string) {
	x.cidr = &s
}

func (x *addressInfo) SetAddress(s string) {
	x.address = &s
}

func (x *addressInfo) SetPrefixLength(s uint32) {
	x.prefixLength = &s
}

func (x *addressInfo) GetIpamPrefix(ctx context.Context, cr vpcv1alpha1.Vp, ipamOptions *IpamOptions) (*string, error) {
	c := x.buildIpamPrefix(cr, ipamOptions)
	if err := x.client.Get(ctx, types.NamespacedName{
		Namespace: cr.GetNamespace(), Name: c.GetName()}, c); err != nil {
		return nil, errors.Wrap(err, errGetPrefix)
	}
	return utils.StringPtr(c.GetName()), nil
}

func (x *addressInfo) CreateIpamPrefix(ctx context.Context, cr vpcv1alpha1.Vp, ipamOptions *IpamOptions) error {
	x.log.Debug("CreateIpamPrefix", "niName", ipamOptions.NetworkInstanceName)
	c := x.buildIpamPrefix(cr, ipamOptions)
	if err := x.client.Apply(ctx, c); err != nil {
		return errors.Wrap(err, errCreatePrefix)
	}
	return nil

}

func (x *addressInfo) DeleteIpamPrefix(ctx context.Context, cr vpcv1alpha1.Vp, ipamOptions *IpamOptions) error {
	c := x.buildIpamPrefix(cr, ipamOptions)
	if err := x.client.Delete(ctx, c); err != nil {
		return errors.Wrap(err, errDeletePrefix)
	}
	return nil
}

func (x *addressInfo) buildIpamPrefix(cr vpcv1alpha1.Vp, ipamOptions *IpamOptions) *ipamv1alpha1.IpamNetworkInstanceIpPrefix {
	siname := strings.ReplaceAll(x.GetSubInterface().GetInterfaceSubInterfaceName(), ".", "-")
	siname = strings.ReplaceAll(siname, "/", "-")

	nodeName := x.GetSubInterface().GetInterface().GetNode().GetName()
	var name string
	//niName := x.GetSubInterface().GetNi().GetName()
	epgName := x.GetSubInterface().GetEpgName()

	if epgName != "" {
		name = strings.Join([]string{cr.GetName(), epgName, siname, ipamOptions.AddressFamily}, "-")
	} else {
		x.log.Debug("irb tracer", "siname", siname, "interfacetype", x.GetSubInterface().GetInterface().GetKind())
		if strings.Contains(siname, "irb") {
			name = strings.Join([]string{cr.GetName(), siname, ipamOptions.AddressFamily}, "-")
		} else {
			name = strings.Join([]string{cr.GetName(), nodeName, siname, ipamOptions.AddressFamily}, "-")
		}
	}
	tags := x.getPurposeTags()
	tags = append(tags, &nddov1.Tag{
		Key:   utils.StringPtr(ipamv1alpha1.KeyAddressFamily),
		Value: utils.StringPtr(ipamOptions.AddressFamily),
	})

	registerName := odns.GetOdnsRegisterName(cr.GetName(),
		[]string{strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), ipamOptions.RegistryName, ipamOptions.NetworkInstanceName},
		[]string{name})

	return &ipamv1alpha1.IpamNetworkInstanceIpPrefix{
		ObjectMeta: metav1.ObjectMeta{
			Name:            registerName,
			Namespace:       cr.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(cr, vpcv1alpha1.VpcGroupVersionKind))},
		},
		Spec: ipamv1alpha1.IpamNetworkInstanceIpPrefixSpec{
			//RegistryName:        &ipamOptions.RegistryName,
			//NetworkInstanceName: &ipamOptions.NetworkInstanceName,
			IpamNetworkInstanceIpPrefix: &ipamv1alpha1.IpamIpamNetworkInstanceIpPrefix{
				AdminState: utils.StringPtr("enable"),
				Prefix:     utils.StringPtr(x.GetCidr()),
				Tag:        tags,
			},
		},
	}
	//r.Spec.Oda = cr.GetOda().Oda
	//return r
}

func (x *addressInfo) getPurposeTags() []*nddov1.Tag {
	tags := x.getPrefixPurposeTags()
	tags = append(tags, &nddov1.Tag{
		Key:   utils.StringPtr(ipamv1alpha1.KeyPurpose),
		Value: utils.StringPtr(x.getPurpose()),
	})
	return tags
}

func (x *addressInfo) getPurpose() string {
	if x.GetSubInterface().GetBridgeDomainName() != "" {
		return x.GetSubInterface().GetBridgeDomainName()
	} else {
		return string(x.GetSubInterface().GetItfceSelectorKind())
	}
}

func (x *addressInfo) getPrefixPurposeTags() []*nddov1.Tag {
	if x.GetSubInterface().GetBridgeDomainName() != "" {
		return make([]*nddov1.Tag, 0)
	} else {
		return x.GetSubInterface().GetItfceSelectorTags()
	}
}

func (x *addressInfo) Print(af, prefix string, n int) {
	fmt.Printf("%s Address IP Prefix %s: %s %s %s %s %d\n", strings.Repeat(" ", n), af, prefix, *x.prefix, *x.cidr, *x.address, *x.prefixLength)
}

func (x *addressInfo) GrpcAllocateEndpointIP(ctx context.Context, cr vpcv1alpha1.Vp, ipamOptions *IpamOptions) (*string, error) {
	req := x.buildGrpcAllocateEndPointIP(cr, ipamOptions)
	reply, err := x.ipamClient.ResourceRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if !reply.Ready {
		return nil, errors.New("grppc ipam allocation server not ready")
	}
	if ipprefix, ok := reply.Data["ip-prefix"]; ok {
		ipprefixVal, err := GetValue(ipprefix)
		if err != nil {
			return nil, err
		}
		switch ipPrefix := ipprefixVal.(type) {
		case string:
			return utils.StringPtr(ipPrefix), nil
		default:
			return nil, errors.New("wrong return data for ipam alocation")
		}
	}
	return nil, nil
}

func (x *addressInfo) GrpcDeAllocateEndpointIP(ctx context.Context, cr vpcv1alpha1.Vp, ipamOptions *IpamOptions) error {
	req := x.buildGrpcAllocateEndPointIP(cr, ipamOptions)
	_, err := x.ipamClient.ResourceRelease(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (x *addressInfo) buildGrpcAllocateEndPointIP(cr vpcv1alpha1.Vp, ipamOptions *IpamOptions) *resourcepb.Request {
	siname := strings.ReplaceAll(x.GetSubInterface().GetInterfaceSubInterfaceName(), ".", "-")
	siname = strings.ReplaceAll(siname, "/", "-")

	nodeName := x.GetSubInterface().GetInterface().GetNode().GetName()

	var name string
	/*
		niName := x.GetSubInterface().GetNi().GetName()
		if x.GetSubInterface().GetInterface().GetKind() == "irb" {
			niName = strings.ReplaceAll(niName, "bridged", "routed")
		}
	*/

	epgName := x.GetSubInterface().GetEpgName()

	if epgName != "" {
		name = strings.Join([]string{epgName, siname, ipamOptions.AddressFamily}, "-")
	} else {
		name = strings.Join([]string{nodeName, siname, ipamOptions.AddressFamily}, "-")
	}

	selectorTags := make(map[string]string)
	for _, tag := range x.getPurposeTags() {
		selectorTags[strings.ReplaceAll(*tag.Key, "/", "-")] = strings.ReplaceAll(*tag.Value, "/", "-")
	}
	selectorTags[ipamv1alpha1.KeyAddressFamily] = ipamOptions.AddressFamily

	var prefix string
	if ipamOptions.AddressFamily == "ipv4" {
		prefix = strings.Join([]string{*x.address, "32"}, "/")
	} else {
		prefix = strings.Join([]string{*x.address, "128"}, "/")
	}

	registerName := odns.GetOdnsRegisterName(cr.GetName(),
		[]string{strings.ToLower(vpcv1alpha1.VpcKindKind), ipamOptions.RegistryName, ipamOptions.NetworkInstanceName},
		[]string{name, ipamOptions.AddressFamily})

	return &resourcepb.Request{
		Namespace:    cr.GetNamespace(),
		RegisterName: registerName,
		//RegistryName:        ipamOptions.RegistryName,
		//Name:                name,
		//NetworkInstanceName: ipamOptions.NetworkInstanceName,
		Kind: "ipam",
		Request: &resourcepb.Req{
			IpPrefix:  prefix,
			Selector:  selectorTags,
			SourceTag: selectorTags, // TODO is this ok or not?
		},
	}
}
