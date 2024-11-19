package webhookcert

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplyWebhookConfigResources(t *testing.T) {
	scheme := runtime.NewScheme()
	ctx := context.Background()
	caBundle := []byte("test-ca-bundle")
	config := Config{
		ServiceName: types.NamespacedName{
			Name:      "test-service",
			Namespace: "test-namespace",
		},
		ValidatingWebhookName: types.NamespacedName{
			Name: "test-validating-webhook",
		},
		MutatingWebhookName: types.NamespacedName{
			Name:      "test-mutating-webhook",
			Namespace: "test-namespace",
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	t.Run("should fail apply webhook config resources", func(t *testing.T) {
		err := applyWebhookConfigResources(ctx, client, caBundle, config)
		require.Error(t, err)
	})
}
