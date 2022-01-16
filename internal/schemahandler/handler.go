package schemahandler

import (
	"sync"

	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/nddo-vpc/pkg/schema"
)

func New(opts ...Option) (Handler, error) {
	s := &handler{
		schema: make(map[string]schema.Schema),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

func (r *handler) WithLogger(log logging.Logger) {
	r.log = log
}

type handler struct {
	log logging.Logger

	mutex  sync.Mutex
	schema map[string]schema.Schema
}

func (r *handler) NewSchema(crName string) schema.Schema {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, ok := r.schema[crName]; !ok {
		r.schema[crName] = schema.NewSchema()
	}
	return r.schema[crName]
}

func (r *handler) DestroySchema(crName string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.schema, crName)
}

func (r *handler) PrintSchemaDevices(crName string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.schema[crName].PrintDevices(crName)
}

func (r *handler) PopulateSchema(crName string) {}
