package webhookcert

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type mockCACertGenerator struct {
	cert, key []byte
}

func (g *mockCACertGenerator) generateCert() ([]byte, []byte, error) {
	return g.cert, g.key, nil
}

func TestProvideCACertKey(t *testing.T) {
	t.Run("should generate new ca cert if no secret found", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().Build()
		fakeCertPEM := []byte{1, 2, 3}
		fakeKeyPEM := []byte{4, 5, 6}
		sut := caCertProvider{
			client:    fakeClient,
			clock:     mockClock{},
			generator: &mockCACertGenerator{cert: fakeCertPEM, key: fakeKeyPEM},
		}

		secretName := types.NamespacedName{Namespace: "default", Name: "ca-cert"}
		certPEM, keyPEM, err := sut.provideCert(context.Background(), secretName)
		require.NoError(t, err)
		require.Equal(t, certPEM, fakeCertPEM)
		require.Equal(t, keyPEM, fakeKeyPEM)
	})

	t.Run("should create secret with ca cert if no secret found", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().Build()
		fakeCertPEM := []byte{1, 2, 3}
		fakeKeyPEM := []byte{4, 5, 6}
		sut := caCertProvider{
			client:    fakeClient,
			clock:     mockClock{},
			generator: &mockCACertGenerator{cert: fakeCertPEM, key: fakeKeyPEM},
		}

		secretName := types.NamespacedName{Namespace: "default", Name: "ca-cert"}
		_, _, err := sut.provideCert(context.Background(), secretName)
		require.NoError(t, err)

		var secret corev1.Secret
		fakeClient.Get(context.Background(), secretName, &secret)
		require.NotNil(t, secret.Data)
		require.Contains(t, secret.Data, "ca.crt")
		require.Contains(t, secret.Data, "ca.key")
		require.Equal(t, secret.Data["ca.crt"], fakeCertPEM)
		require.Equal(t, secret.Data["ca.key"], fakeKeyPEM)
	})

	t.Run("should create new secret with ca cert if existing secret contains invalid keys", func(t *testing.T) {
		fakeCertPEM := []byte{1, 2, 3}
		fakeKeyPEM := []byte{4, 5, 6}
		fakeClient := fake.NewClientBuilder().WithObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ca-cert",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"ca.certificate": {7, 8, 9},
				"ca.key":         {10, 11, 12},
			},
		}).Build()
		sut := caCertProvider{
			client:    fakeClient,
			clock:     mockClock{},
			generator: &mockCACertGenerator{cert: fakeCertPEM, key: fakeKeyPEM},
		}

		secretName := types.NamespacedName{Namespace: "default", Name: "ca-cert"}
		_, _, err := sut.provideCert(context.Background(), secretName)
		require.NoError(t, err)

		var secret corev1.Secret
		fakeClient.Get(context.Background(), secretName, &secret)
		require.NotNil(t, secret.Data)
		require.Contains(t, secret.Data, "ca.crt")
		require.Contains(t, secret.Data, "ca.key")
		require.Equal(t, secret.Data["ca.crt"], fakeCertPEM)
		require.Equal(t, secret.Data["ca.key"], fakeKeyPEM)
	})

	t.Run("should create new secret with ca cert if existing secret contains cert expiring soon", func(t *testing.T) {
		now := time.Now()
		fakeExpiringCertPEM, fakeExpiringKeyPEM := fakeCACert(now.Add(-1 * duration365d))
		fakeNewCertPEM, fakeNewKeyPEM := fakeCACert(now)
		fakeClient := fake.NewClientBuilder().WithObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ca-cert",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"ca.crt": fakeExpiringCertPEM,
				"ca.key": fakeExpiringKeyPEM,
			},
		}).Build()
		sut := caCertProvider{
			client:    fakeClient,
			clock:     mockClock{t: now},
			generator: &mockCACertGenerator{cert: fakeNewCertPEM, key: fakeNewKeyPEM},
		}

		secretName := types.NamespacedName{Namespace: "default", Name: "ca-cert"}
		certPEM, keyPEM, err := sut.provideCert(context.Background(), secretName)
		require.NoError(t, err)
		require.Equal(t, fakeNewCertPEM, certPEM)
		require.Equal(t, fakeNewKeyPEM, keyPEM)

		var secret corev1.Secret
		fakeClient.Get(context.Background(), secretName, &secret)
		require.NotNil(t, secret.Data)
		require.Contains(t, secret.Data, "ca.crt")
		require.Contains(t, secret.Data, "ca.key")
		require.Equal(t, secret.Data["ca.crt"], fakeNewCertPEM)
		require.Equal(t, secret.Data["ca.key"], fakeNewKeyPEM)
	})

	t.Run("should return ca cert from existing secret if not expired", func(t *testing.T) {
		fakeCertPEM := []byte{1, 2, 3}
		fakeKeyPEM := []byte{4, 5, 6}
		fakeClient := fake.NewClientBuilder().WithObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ca-cert",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"ca.crt": fakeCertPEM,
				"ca.key": fakeKeyPEM,
			},
		}).Build()
		sut := caCertProvider{
			client:    fakeClient,
			clock:     mockClock{},
			generator: &mockCACertGenerator{cert: fakeCertPEM, key: fakeKeyPEM},
		}

		secretName := types.NamespacedName{Namespace: "default", Name: "ca-cert"}
		_, _, err := sut.provideCert(context.Background(), secretName)
		require.NoError(t, err)

		var secret corev1.Secret
		fakeClient.Get(context.Background(), secretName, &secret)
		require.NotNil(t, secret.Data)
		require.Contains(t, secret.Data, "ca.crt")
		require.Contains(t, secret.Data, "ca.key")
		require.Equal(t, secret.Data["ca.crt"], fakeCertPEM)
		require.Equal(t, secret.Data["ca.key"], fakeKeyPEM)
	})
}

func fakeCACert(now time.Time) ([]byte, []byte) {
	generator := &caCertGeneratorImpl{clock: mockClock{t: now}}
	cert, key, _ := generator.generateCert()
	return cert, key
}
