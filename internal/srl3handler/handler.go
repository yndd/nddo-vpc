package srl3handler

import (
	"context"
	"strings"
	"sync"

	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/nddo-runtime/pkg/resource"
	schema "github.com/yndd/nddp-srl3/pkg/srl3/v1alpha1"

	//topov1alpha1 "github.com/yndd/nddr-topo-registry/apis/topo/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(opts ...Option) Handler {
	s := &handler{
		schema: make(map[string]schema.Schema),
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
	client resource.ClientApplicator

	mutex  sync.Mutex
	schema map[string]schema.Schema
}

func (r *handler) Init(crName string) schema.Schema {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, ok := r.schema[crName]; !ok {
		r.schema[crName] = schema.NewSchema(r.client)
	}
	return r.schema[crName]
}

func (r *handler) Destroy(crName string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.schema, crName)
}

func (r *handler) Deploy(ctx context.Context, mg resource.Managed, labels map[string]string) error {
	crName := getCrName(mg)
	s := r.Init(crName)

	if err := s.Deploy(ctx, mg, labels); err != nil {
		return err
	}
	return nil
}

func (r *handler) Validate(ctx context.Context, mg resource.Managed) error {
	ds := r.Init("dummy")
	//ds.InitializeDummySchema()
	resources, err := ds.List(ctx, mg)
	if err != nil {
		return err
	}
	for kind, res := range resources {
		for resName := range res {
			r.log.Debug("active resources", "kind", kind, "resource name", resName)
		}
	}

	crName := getCrName(mg)
	s := r.Init(crName)
	validatedResources, err := s.Validate(ctx, mg, resources)
	if err != nil {
		return err
	}
	for kind, res := range validatedResources {
		for resName := range res {
			r.log.Debug("validated resources", "kind", kind, "resource name", resName)
		}
	}

	if len(validatedResources) > 0 {
		r.log.Debug("resources to be deleted", "resources", validatedResources)
		if err := ds.Delete(ctx, mg, resources); err != nil {
			return err
		}
	}

	return nil
}

func (r *handler) Print(crName string) {
	s := r.Init(crName)
	if err := s.Print(crName); err != nil {
		r.log.Debug("print errir", "error", err)
	}
}

func getCrName(mg resource.Managed) string {
	return strings.Join([]string{mg.GetNamespace(), mg.GetName()}, ".")
}