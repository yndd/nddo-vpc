package schemahandler

import (
	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/nddo-vpc/pkg/schema"
)

// Option can be used to manipulate Options.
type Option func(Handler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) Option {
	return func(s Handler) {
		s.WithLogger(log)
	}
}

type Handler interface {
	WithLogger(log logging.Logger)
	NewSchema(string) schema.Schema
	DestroySchema(string)

	//GetSchemaLinks(string) map[string]schema.Link
	//GetSchemaDevices(string) map[string]schema.Device
	//GetSchemaNis(string) map[string]schema.Ni
	PrintSchemaDevices(string)
}
