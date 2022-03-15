package nihandler

import (
	"context"

	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/ndda-network/pkg/ndda/niinfo"
	"github.com/yndd/nddo-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NiOptions struct {
	RegistryName        string
	NetworkInstanceName string
}

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
	GetNiRegister(ctx context.Context, mg resource.Managed, niOptions *niinfo.NiInfo) (*uint32, error)
	CreateNiRegister(ctx context.Context, mg resource.Managed, niOptions *niinfo.NiInfo) error
	DeleteNiRegister(ctx context.Context, mg resource.Managed, niOptions *niinfo.NiInfo) error
}
