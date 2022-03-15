package srl3handler

import (
	"context"

	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/nddo-runtime/pkg/resource"
	srl3schema "github.com/yndd/nddp-srl3/pkg/srl3/v1alpha1"
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
	Init(crName string) srl3schema.Schema
	Destroy(crName string)
	Deploy(ctx context.Context, mg resource.Managed, labels map[string]string) error
	Validate(ctx context.Context, mg resource.Managed) error
	Print(string)
}
