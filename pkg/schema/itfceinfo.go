package schema

import "reflect"

type ItfceInfo interface {
	//GetEpgName() string
	//GetNiName() string
	//GetNiKind() string
	GetItfceName() string
	GetItfceIndex() uint32
	GetItfceKind() string
	//GetNodeName() string
	GetVlanId() uint32
	GetIpv4Prefixes() []*string
	GetIpv6Prefixes() []*string
	//GetItfceSelectorKind() nddov1.InterfaceSelectorKind
	//GetItfceSelectorTags() []*nddov1.Tag
	//GetBridgeDomainName() string
	//GetGlobalParamaters() GlobalParameters
	//SetEpgName(string)
	//SetNiName(string)
	//SetNiKind(string)
	SetItfceName(string)
	SetItfceIndex(uint32)
	SetItfceKind(string)
	//SetNodeName(string)
	SetVlanId(s uint32)
	SetIpv4Prefixes([]*string)
	SetIpv6Prefixes([]*string)
	//SetItfceSelectorKind(nddov1.InterfaceSelectorKind)
	//SetItfceSelectorTags([]*nddov1.Tag)
	//SetBridgeDomainName(string)
	//IsEpgBased() bool
}

type ItfceInfoOption func(*itfceInfo)

/*
func WithEpgName(s string) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.epgName = &s
	}
}

func WithNiName(s string) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.niName = &s
	}
}

func WithNiKind(s string) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.niKind = &s
	}
}
*/

func WithItfceName(s string) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.itfceName = &s
	}
}

func WithItfceIndex(s uint32) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.itfceIndex = &s
	}
}

func WithItfceKind(s string) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.itfceKind = &s
	}
}

/*
func WithNodeName(s string) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.nodeName = &s
	}
}
*/

func WithVlan(s uint32) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.vlanID = &s
	}
}

func WithIpv4Prefixes(s []*string) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.ipv4Prefixes = s
	}
}

func WithIpv6Prefixes(s []*string) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.ipv6Prefixes = s
	}
}

/*
func WithItfceSelectorKind(s nddov1.InterfaceSelectorKind) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.itfceSelectorKind = s
	}
}

func WithItfceSelectorTags(s []*nddov1.Tag) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.itfceSelectorTags = s
	}
}

func WithBridgeDomainName(s string) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.bdName = &s
	}
}

func WithRegister(s map[string]string) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.globalParameters.SetRegister(s)
	}
}

func WithAddressAllocationStrategy(s *nddov1.AddressAllocationStrategy) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.globalParameters.SetAddressAllocationStrategy(s)
	}
}

func WithResourceClient(s string, c resourcepb.ResourceClient) ItfceInfoOption {
	return func(r *itfceInfo) {
		r.globalParameters.SetResourceClient(s, c)
	}
}
*/

func NewItfceInfo(opts ...ItfceInfoOption) ItfceInfo {
	i := &itfceInfo{
		ipv4Prefixes: make([]*string, 0),
		ipv6Prefixes: make([]*string, 0),
		//itfceSelectorTags: make([]*nddov1.Tag, 0),
		//globalParameters: &globalParameters{
		//	resourceClient: make(map[string]resourcepb.ResourceClient),
		//},
	}

	for _, f := range opts {
		f(i)
	}

	return i
}

type itfceInfo struct {
	//epgName           *string
	//niName            *string
	//niKind            *string
	//nodeName          *string
	itfceName    *string
	itfceIndex   *uint32
	itfceKind    *string
	vlanID       *uint32
	ipv4Prefixes []*string
	ipv6Prefixes []*string
	//itfceSelectorKind nddov1.InterfaceSelectorKind // the origin interface selector kind
	//itfceSelectorTags []*nddov1.Tag                // used to keep track of the origin how the selection was done, we add this to the prefixes
	//bdName            *string                      // used by IRB interfaces for tagging the prefixes
	//globalParameters  GlobalParameters
}

/*
func (x *itfceInfo) GetEpgName() string {
	return *x.epgName
}
*/

func (x *itfceInfo) GetItfceName() string {
	return *x.itfceName
}

func (x *itfceInfo) GetItfceIndex() uint32 {
	return *x.itfceIndex
}

func (x *itfceInfo) GetItfceKind() string {
	return *x.itfceKind
}

/*
func (x *itfceInfo) GetNiName() string {
	return *x.niName
}

func (x *itfceInfo) GetNiKind() string {
	return *x.niKind
}

func (x *itfceInfo) GetNodeName() string {
	return *x.nodeName
}
*/

func (x *itfceInfo) GetVlanId() uint32 {
	if reflect.ValueOf(x.vlanID).IsZero() {
		return 9999
	}
	return *x.vlanID
}

func (x *itfceInfo) GetIpv4Prefixes() []*string {
	return x.ipv4Prefixes
}

func (x *itfceInfo) GetIpv6Prefixes() []*string {
	return x.ipv6Prefixes
}

/*
func (x *itfceInfo) GetItfceSelectorKind() nddov1.InterfaceSelectorKind {
	return x.itfceSelectorKind
}

func (x *itfceInfo) GetItfceSelectorTags() []*nddov1.Tag {
	return x.itfceSelectorTags
}

func (x *itfceInfo) GetBridgeDomainName() string {
	if reflect.ValueOf(x.bdName).IsZero() {
		return ""
	}
	return *x.bdName
}

func (x *itfceInfo) GetGlobalParamaters() GlobalParameters {
	return x.globalParameters
}
*/

/*
func (x *itfceInfo) SetEpgName(s string) {
	x.epgName = &s
}

func (x *itfceInfo) SetNiName(s string) {
	x.niName = &s
}

func (x *itfceInfo) SetNiKind(s string) {
	x.niKind = &s
}
*/
func (x *itfceInfo) SetItfceName(s string) {
	x.itfceName = &s
}

func (x *itfceInfo) SetItfceIndex(s uint32) {
	x.itfceIndex = &s
}

func (x *itfceInfo) SetItfceKind(s string) {
	x.itfceKind = &s
}

/*
func (x *itfceInfo) SetNodeName(s string) {
	x.nodeName = &s
}
*/

func (x *itfceInfo) SetVlanId(s uint32) {
	x.vlanID = &s
}

func (x *itfceInfo) SetIpv4Prefixes(s []*string) {
	x.ipv4Prefixes = s
}

func (x *itfceInfo) SetIpv6Prefixes(s []*string) {
	x.ipv6Prefixes = s
}

/*
func (x *itfceInfo) SetItfceSelectorKind(s nddov1.InterfaceSelectorKind) {
	x.itfceSelectorKind = s
}

func (x *itfceInfo) SetItfceSelectorTags(s []*nddov1.Tag) {
	x.itfceSelectorTags = s
}

func (x *itfceInfo) SetBridgeDomainName(s string) {
	x.bdName = &s
}

func (x *itfceInfo) IsEpgBased() bool {
	return *x.epgName != ""
}
*/
