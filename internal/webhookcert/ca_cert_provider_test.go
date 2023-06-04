package webhookcert

import (
	"context"
	"errors"
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

func TestProvideCACert(t *testing.T) {
	t.Run("should generate new ca cert if no secret found", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().Build()
		fakeCertPEM := []byte{1, 2, 3}
		fakeKeyPEM := []byte{4, 5, 6}
		sut := caCertProviderImpl{
			client:    fakeClient,
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
		sut := caCertProviderImpl{
			client:    fakeClient,
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
		sut := caCertProviderImpl{
			client:    fakeClient,
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
		fakeExpiringCertPEM, fakeExpiringKeyPEM := []byte{1, 2, 3}, []byte{4, 5, 6}
		fakeNewCertPEM, fakeNewKeyPEM := []byte{7, 8, 9}, []byte{10, 11, 12}
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
		sut := caCertProviderImpl{
			client:        fakeClient,
			expiryChecker: &mockCertExpiryChecker{certValid: false},
			generator:     &mockCACertGenerator{cert: fakeNewCertPEM, key: fakeNewKeyPEM},
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

	t.Run("should create new secret with ca cert if expiry check fails", func(t *testing.T) {
		fakeExpiringCertPEM, fakeExpiringKeyPEM := []byte{1, 2, 3}, []byte{4, 5, 6}
		fakeNewCertPEM, fakeNewKeyPEM := []byte{7, 8, 9}, []byte{10, 11, 12}
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
		sut := caCertProviderImpl{
			client:        fakeClient,
			expiryChecker: &mockCertExpiryChecker{err: errors.New("failed")},
			generator:     &mockCACertGenerator{cert: fakeNewCertPEM, key: fakeNewKeyPEM},
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
		sut := caCertProviderImpl{
			client:        fakeClient,
			expiryChecker: &mockCertExpiryChecker{certValid: true},
			generator:     &mockCACertGenerator{cert: fakeCertPEM, key: fakeKeyPEM},
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

func generateCACertKey(creationTime time.Time) ([]byte, []byte) {
	generator := &caCertGeneratorImpl{clock: mockClock{t: creationTime}}
	cert, key, _ := generator.generateCert()
	return cert, key
}

func generateCACert(creationTime time.Time) []byte {
	certPEM, _ := generateCACertKey(creationTime)
	return certPEM
}

func generateServerCert(caCert, caKey []byte, creationTime time.Time) []byte {
	generator := &serverCertGeneratorImpl{clock: mockClock{t: creationTime}}
	cert, _, _ := generator.generateCert(serverCertConfig{caCertPEM: caCert, caKeyPEM: caKey})
	return cert
}
