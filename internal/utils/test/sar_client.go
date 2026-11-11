package test

import (
	"context"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// SARAllowingClient returns a fake client whose SubjectAccessReview handler always grants access.
func SARAllowingClient(t *testing.T) client.Client {
	t.Helper()

	return sarClient(t, true, "")
}

// SARDenyingClient returns a fake client whose SubjectAccessReview handler always denies access.
func SARDenyingClient(t *testing.T) client.Client {
	t.Helper()

	return sarClient(t, false, "access denied")
}

func sarClient(t *testing.T, allowed bool, reason string) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := authorizationv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add authorizationv1 to scheme: %v", err)
	}

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if sar, ok := obj.(*authorizationv1.SubjectAccessReview); ok {
					sar.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: allowed, Reason: reason}
					return nil
				}

				return c.Create(ctx, obj, opts...)
			},
		}).
		Build()
}
