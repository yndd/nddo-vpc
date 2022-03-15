package nihandler

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/ndd-runtime/pkg/meta"
	"github.com/yndd/ndd-runtime/pkg/utils"
	"github.com/yndd/ndda-network/pkg/ndda/niinfo"
	nddov1 "github.com/yndd/nddo-runtime/apis/common/v1"
	"github.com/yndd/nddo-runtime/pkg/odns"
	"github.com/yndd/nddo-runtime/pkg/resource"
	niv1alpha1 "github.com/yndd/nddr-ni-registry/apis/ni/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	NiPrefix      = "infra"
	NiAllocPrefix = "alloc-ni"

	errCreateNiRegister      = "cannot create ni register"
	errDeleteNiRegister      = "cannot delete ni register"
	errGetNiRegister         = "cannot get ni register"
	errUnavailableNiRegister = "ni register unavailable"
)

func New(opts ...Option) Handler {
	s := &handler{}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

type handler struct {
	log logging.Logger
	// kubernetes
	client resource.ClientApplicator
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

func (r *handler) GetNiRegister(ctx context.Context, mg resource.Managed, niInfo *niinfo.NiInfo) (*uint32, error) {
	o := buildNiRegister(mg, niInfo)
	if err := r.client.Get(ctx, types.NamespacedName{
		Namespace: mg.GetNamespace(), Name: o.GetName()}, o); err != nil {
		return nil, errors.Wrap(err, errGetNiRegister)
	}
	if o.GetCondition(niv1alpha1.ConditionKindReady).Status == corev1.ConditionTrue {
		if ni, ok := o.HasNi(); ok {
			return &ni, nil
		}
		r.log.Debug("strange NI alloc ready but no NI allocated")
		return nil, errors.Errorf("%s: %s", errUnavailableNiRegister, "strange NI register ready but no NI allocated")
	}
	return nil, errors.Errorf("%s: %s", errUnavailableNiRegister, o.GetCondition(niv1alpha1.ConditionKindReady).Message)
}

func (r *handler) CreateNiRegister(ctx context.Context, mg resource.Managed, niInfo *niinfo.NiInfo) error {
	o := buildNiRegister(mg, niInfo)
	if err := r.client.Apply(ctx, o); err != nil {
		return errors.Wrap(err, errCreateNiRegister)
	}
	return nil
}

func (r *handler) DeleteNiRegister(ctx context.Context, mg resource.Managed, niInfo *niinfo.NiInfo) error {
	o := buildNiRegister(mg, niInfo)
	if err := r.client.Delete(ctx, o); err != nil {
		return errors.Wrap(err, errDeleteNiRegister)
	}
	return nil
}

func buildNiRegister(mg resource.Managed, niInfo *niinfo.NiInfo) *niv1alpha1.Register {
	registerName := odns.GetOdnsRegisterName(mg.GetName(),
		[]string{strings.ToLower(mg.GetObjectKind().GroupVersionKind().Kind), niInfo.GetNiRegistry()},
		[]string{strings.Join([]string{"register", niInfo.GetNiName()}, "-")},
	)

	return &niv1alpha1.Register{
		ObjectMeta: metav1.ObjectMeta{
			Name:            registerName,
			Namespace:       mg.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(mg, mg.GetObjectKind().GroupVersionKind()))},
		},
		Spec: niv1alpha1.RegisterSpec{
			Register: &niv1alpha1.NiRegister{
				Selector: []*nddov1.Tag{
					{Key: utils.StringPtr(niv1alpha1.NiSelectorKey), Value: utils.StringPtr(niInfo.GetNiName())},
				},
				SourceTag: []*nddov1.Tag{},
			},
		},
	}
}
