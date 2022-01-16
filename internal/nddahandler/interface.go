package nddahandler

/*
import (
	"github.com/yndd/ndd-runtime/pkg/logging"
	nddaschema "github.com/yndd/ndda-network/pkg/ndda/v1alpha1"
	nddov1 "github.com/yndd/nddo-runtime/apis/common/v1"
	"github.com/yndd/nddo-vpc/pkg/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Option can be used to manipulate Options.
type Option func(Handler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) Option {
	return func(s Handler) {
		s.WithLogger(log)
	}
}

func WithClient(c client.Client) Option {
	return func(s Handler) {
		s.WithClient(c)
	}
}

type Handler interface {
	WithLogger(log logging.Logger)
	WithClient(c client.Client)
	NewSchema(crName string) nddaschema.Schema
	DestroySchema(crName string)
	PopulateSchema(crName, deviceName string, itfceInfo schema.ItfceInfo, niInfo *schema.DeviceNetworkInstanceData, addressAllocationStrategy *nddov1.AddressAllocationStrategy) error

	PrintSchemaDevices(string)
}
*/
