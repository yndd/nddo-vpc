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

/*
func (r *handler) GetSchemaLinks(crName string) map[string]schema.Link {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.schema[crName].GetLinks()
}
*/

/*
func (r *handler) GetSchemaDevices(crName string) map[string]schema.Device {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.schema[crName].GetDevices()
}
*/

/*
func (r *handler) GetSchemaNis(crName string) map[string]schema.Ni {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.schema[crName].GetNis()
}
*/


func (r *handler) PrintSchemaDevices(crName string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.schema[crName].PrintDevices(crName)
}
