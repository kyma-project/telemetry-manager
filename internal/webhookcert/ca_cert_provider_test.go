package webhookcert

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestProvideCACertKey(t *testing.T) {
	t.Run("should generateCert new if ca secret not found", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().Build()
		sut := newCACertProvider(fakeClient)

		certPEM, keyPEM, err := sut.provideCert(context.Background(), types.NamespacedName{
			Namespace: "default",
			Name:      "ca-cert",
		})
		require.NoError(t, err)
		require.NotNil(t, certPEM)
		require.NotNil(t, keyPEM)
	})
}
