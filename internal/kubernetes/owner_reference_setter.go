package kubernetes

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type noWatch struct {
	client.Client
}

func (n *noWatch) Watch(ctx context.Context, list client.ObjectList, opts ...client.ListOption) (watch.Interface, error) {
	return nil, nil
}

func NewOwnerReferenceSetter(interceptedClient client.Client, owner metav1.Object) client.Client {
	return interceptor.NewClient(&noWatch{Client: interceptedClient}, interceptor.Funcs{
		Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if err := controllerutil.SetOwnerReference(owner, obj, interceptedClient.Scheme()); err != nil {
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
			return client.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if err := controllerutil.SetOwnerReference(owner, obj, interceptedClient.Scheme()); err != nil {
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
			return client.Update(ctx, obj, opts...)
		},
		Patch: func(ctx context.Context, client client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if err := controllerutil.SetOwnerReference(owner, obj, interceptedClient.Scheme()); err != nil {
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
			return client.Patch(ctx, obj, patch, opts...)
		},
	})
}
