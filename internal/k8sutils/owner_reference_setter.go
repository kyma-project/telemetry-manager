package k8sutils

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// NewOwnerReferenceSetter wraps an existing Kubernetes client with additional functionality
// to set the owner reference for objects before they are created or updated.
// It returns a new client that, when used for Create or Update operations, will automatically
// set the given owner as the owner of the object being operated on.
func NewOwnerReferenceSetter(interceptedClient client.Client, owner metav1.Object) client.Client {
	return interceptor.NewClient(&noopWatchClient{Client: interceptedClient}, interceptor.Funcs{
		Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			err := setOwnerReference(owner, obj, interceptedClient.Scheme())
			if err != nil {
				return err
			}
			return client.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			err := setOwnerReference(owner, obj, interceptedClient.Scheme())
			if err != nil {
				return err
			}
			return client.Update(ctx, obj, opts...)
		},
	})
}

type noopWatchClient struct {
	client.Client
}

func (n *noopWatchClient) Watch(_ context.Context, _ client.ObjectList, _ ...client.ListOption) (watch.Interface, error) {
	return nil, nil
}

func setOwnerReference(owner metav1.Object, ownee client.Object, scheme *runtime.Scheme) error {
	if err := controllerutil.SetOwnerReference(owner, ownee, scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}
	return nil
}
