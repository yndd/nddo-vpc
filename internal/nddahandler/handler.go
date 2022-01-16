package nddahandler

/*
import (
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/ndd-runtime/pkg/utils"
	nddav1alpha1 "github.com/yndd/ndda-network/apis/ndda/v1alpha1"
	nddaschema "github.com/yndd/ndda-network/pkg/ndda/v1alpha1"
	nddov1 "github.com/yndd/nddo-runtime/apis/common/v1"
	"github.com/yndd/nddo-runtime/pkg/resource"
	"github.com/yndd/nddo-vpc/pkg/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(opts ...Option) Handler {
	s := &handler{
		schema: make(map[string]nddaschema.Schema),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (r *handler) WithLogger(log logging.Logger) {
	r.log = log
}

func (r *handler) WithClient(c client.Client) {
	r.client = resource.ClientApplicator{
		Client:     c,
		Applicator: resource.NewAPIPatchingApplicator(c),
	}
}

type handler struct {
	log logging.Logger
	// kubernetes
	client client.Client

	mutex  sync.Mutex
	schema map[string]nddaschema.Schema
}

func (r *handler) NewSchema(crName string) nddaschema.Schema {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, ok := r.schema[crName]; !ok {
		r.schema[crName] = nddaschema.NewSchema()
	}
	return r.schema[crName]
}

func (r *handler) DestroySchema(crName string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.schema, crName)
}

func (r *handler) PrintSchemaDevices(crName string) {
	//r.mutex.Lock()
	//defer r.mutex.Unlock()
	//r.schema[crName].PrintDevices(crName)
}

func (r *handler) PopulateSchema(crName, deviceName string, itfceInfo schema.ItfceInfo, niInfo *schema.DeviceNetworkInstanceData, addressAllocationStrategy *nddov1.AddressAllocationStrategy) error {
	s := r.NewSchema(crName)

	d := s.NewDevice(deviceName)

	i := d.NewInterface(itfceInfo.GetItfceName())

	var si nddaschema.InterfaceSubinterface
	//var si *DeviceInterfaceSubInterfaceData
	if itfceInfo.GetItfceKind() == "interface" {
		si = i.NewInterfaceSubinterface(strconv.Itoa(int(itfceInfo.GetVlanId())))
		if len(itfceInfo.GetIpv4Prefixes()) > 0 {
			ipv4 := make([]*nddav1alpha1.InterfaceSubinterfaceIpv4, 0)
			for _, prefix := range itfceInfo.GetIpv4Prefixes() {

				ad, err := getIPv4Info(prefix, addressAllocationStrategy)
				if err != nil {
					return err
				}
				ipv4 = append(ipv4, ad)
			}
			si.Update(&nddav1alpha1.InterfaceSubinterface{
				Index: utils.StringPtr(strconv.Itoa(int(itfceInfo.GetVlanId()))),
				Config: &nddav1alpha1.InterfaceSubinterfaceConfig{
					Index:       utils.Uint32Ptr(itfceInfo.GetVlanId()),
					Kind:        nddav1alpha1.InterfaceSubinterfaceKind_BRIDGED, // should come from itfceInfo.GetItfceKind()
					OuterVlanId: utils.Uint16Ptr(uint16(itfceInfo.GetVlanId())), // user defined, what to do when user is not defining them?
					InnerVlanId: utils.Uint16Ptr(uint16(itfceInfo.GetVlanId())), // user defined, what to do when user is not defining them?

				},
				Ipv4: ipv4,
			})
		}

	} else { // vxlan or irb
		si = i.NewInterfaceSubinterface("0")
		si.Update(&nddav1alpha1.InterfaceSubinterface{
			Index: utils.StringPtr(strconv.Itoa(int(itfceInfo.GetVlanId()))),
			Config: &nddav1alpha1.InterfaceSubinterfaceConfig{
				Index: utils.Uint32Ptr(itfceInfo.GetVlanId()),
				Kind:  nddav1alpha1.InterfaceSubinterfaceKind_BRIDGED, // should come from itfceInfo.GetItfceKind()
			},
		})
	}

	ni := d.NewNetworkInstance(*niInfo.Name)

	ni.Update(&nddav1alpha1.NetworkInstance{
		Name: utils.StringPtr(strings.Join([]string{*niInfo.Name, *niInfo.Kind}, "-")),
		Config: &nddav1alpha1.NetworkInstanceConfig{
			Name: utils.StringPtr(strings.Join([]string{*niInfo.Name, *niInfo.Kind}, "-")),
			Kind: nddav1alpha1.NetworkInstanceKind_BRIDGED, //utils.StringPtr(*niInfo.Kind),
			Interface: []*nddav1alpha1.NetworkInstanceConfigInterface{
				{Name: utils.StringPtr("to be added")},
			},
			Index: utils.Uint32Ptr(*niInfo.Index), // to be allocated
		},
	})


	return nil
}
*/

/*
func getIPv4Info(prefix *string, addressAllocation *nddov1.AddressAllocationStrategy) (*nddav1alpha1.InterfaceSubinterfaceIpv4, error) {
	ip, n, err := net.ParseCIDR(*prefix)
	if err != nil {
		return nil, err
	}
	ipMask, _ := n.Mask.Size()
	if ip.String() == n.IP.String() && ipMask != 31 {
		switch addressAllocation.GetGatewayAllocation() {
		case nddov1.GatewayAllocationLast.String():
			ipAddr, err := GetLastIP(n)
			if err != nil {
				return nil, err
			}
			return &nddav1alpha1.InterfaceSubinterfaceIpv4{
				IpPrefix: utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				Config: &nddav1alpha1.InterfaceSubinterfaceIpv4Config{
					IpPrefix:     utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
					IpCidr:       utils.StringPtr(n.String()),
					PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
					IpAddress:    utils.StringPtr(ipAddr.String()),
				},
			}, nil
		default:
			//case nddov1.GatewayAllocationFirst:
			ipAddr, err := GetFirstIP(n)
			if err != nil {
				return nil, err
			}
			return &nddav1alpha1.InterfaceSubinterfaceIpv4{
				IpPrefix: utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
				Config: &nddav1alpha1.InterfaceSubinterfaceIpv4Config{
					IpPrefix:     utils.StringPtr(strings.Join([]string{ipAddr.String(), strconv.Itoa(ipMask)}, "/")),
					IpCidr:       utils.StringPtr(n.String()),
					PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
					IpAddress:    utils.StringPtr(ipAddr.String()),
				},
			}, nil
		}
	}
	return &nddav1alpha1.InterfaceSubinterfaceIpv4{
		IpPrefix: prefix,
		Config: &nddav1alpha1.InterfaceSubinterfaceIpv4Config{
			IpPrefix:     prefix,
			IpCidr:       utils.StringPtr(n.String()),
			PrefixLength: utils.Uint32Ptr(uint32(ipMask)),
			IpAddress:    utils.StringPtr(ip.String()),
		},
	}, nil

}

// GetLastIP returns subnet's last IP
func GetLastIP(subnet *net.IPNet) (net.IP, error) {
	size := RangeSize(subnet)
	if size <= 0 {
		return nil, fmt.Errorf("can't get range size of subnet. subnet: %q", subnet)
	}
	return GetIndexedIP(subnet, int(size-1))
}

// GetFirstIP returns subnet's last IP
func GetFirstIP(subnet *net.IPNet) (net.IP, error) {
	return GetIndexedIP(subnet, 1)
}

// RangeSize returns the size of a range in valid addresses.
func RangeSize(subnet *net.IPNet) int64 {
	ones, bits := subnet.Mask.Size()
	if bits == 32 && (bits-ones) >= 31 || bits == 128 && (bits-ones) >= 127 {
		return 0
	}
	// For IPv6, the max size will be limited to 65536
	// This is due to the allocator keeping track of all the
	// allocated IP's in a bitmap. This will keep the size of
	// the bitmap to 64k.
	if bits == 128 && (bits-ones) >= 16 {
		return int64(1) << uint(16)
	}
	return int64(1) << uint(bits-ones)
}

// GetIndexedIP returns a net.IP that is subnet.IP + index in the contiguous IP space.
func GetIndexedIP(subnet *net.IPNet, index int) (net.IP, error) {
	ip := addIPOffset(bigForIP(subnet.IP), index)
	if !subnet.Contains(ip) {
		return nil, fmt.Errorf("can't generate IP with index %d from subnet. subnet too small. subnet: %q", index, subnet)
	}
	return ip, nil
}

// addIPOffset adds the provided integer offset to a base big.Int representing a
// net.IP
func addIPOffset(base *big.Int, offset int) net.IP {
	return net.IP(big.NewInt(0).Add(base, big.NewInt(int64(offset))).Bytes())
}

// bigForIP creates a big.Int based on the provided net.IP
func bigForIP(ip net.IP) *big.Int {
	b := ip.To4()
	if b == nil {
		b = ip.To16()
	}
	return big.NewInt(0).SetBytes(b)
}
*/
