package webhookcert

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockServerCertStorage struct {
	certPEM, keyPEM []byte
}

func (s *mockServerCertStorage) load() ([]byte, []byte, error) {
	return s.certPEM, s.keyPEM, nil
}

func (s *mockServerCertStorage) save(certPEM, keyPEM []byte) error {
	s.certPEM = certPEM
	s.keyPEM = keyPEM
	return nil
}

type mockServerCertGenerator struct {
	cert, key []byte
}

func (g *mockServerCertGenerator) generateCert(serverCertConfig) (certPEM []byte, keyPEM []byte, err error) {
	return g.cert, g.key, nil
}

func TestProvideServerCert(t *testing.T) {
	t.Run("should generate new server cert if no cert found in storage", func(t *testing.T) {
		fakeCertPEM := []byte{1, 2, 3}
		fakeKeyPEM := []byte{4, 5, 6}
		sut := serverCertProviderImpl{
			storage:   &mockServerCertStorage{},
			checker:   &mockCertExpiryChecker{},
			generator: &mockServerCertGenerator{cert: fakeCertPEM, key: fakeKeyPEM},
		}

		certPEM, keyPEM, err := sut.provideCert(context.Background(), serverCertConfig{})
		require.NoError(t, err)
		require.Equal(t, certPEM, fakeCertPEM)
		require.Equal(t, keyPEM, fakeKeyPEM)
	})

	t.Run("should store new server cert if no cert found in storage", func(t *testing.T) {
		fakeCertPEM, fakeKeyPEM := []byte{1, 2, 3}, []byte{4, 5, 6}
		mockStorage := &mockServerCertStorage{}
		sut := serverCertProviderImpl{
			storage:   mockStorage,
			checker:   &mockCertExpiryChecker{},
			generator: &mockServerCertGenerator{cert: fakeCertPEM, key: fakeKeyPEM},
		}

		certPEM, keyPEM, err := sut.provideCert(context.Background(), serverCertConfig{})
		require.NoError(t, err)
		require.Equal(t, mockStorage.certPEM, certPEM)
		require.Equal(t, mockStorage.keyPEM, keyPEM)
	})

	t.Run("should store new server cert if existing cert expiring soon", func(t *testing.T) {
		fakeExpiringCertPEM, fakeExpiringKeyPEM := []byte{1, 2, 3}, []byte{4, 5, 6}
		fakeNewCertPEM, fakeNewKeyPEM := []byte{7, 8, 9}, []byte{10, 11, 12}
		mockStorage := &mockServerCertStorage{
			certPEM: fakeExpiringCertPEM,
			keyPEM:  fakeExpiringKeyPEM,
		}
		sut := serverCertProviderImpl{
			storage:   mockStorage,
			checker:   &mockCertExpiryChecker{certValid: false},
			generator: &mockServerCertGenerator{cert: fakeNewCertPEM, key: fakeNewKeyPEM},
		}

		_, _, err := sut.provideCert(context.Background(), serverCertConfig{})
		require.NoError(t, err)
		require.Equal(t, mockStorage.certPEM, fakeNewCertPEM)
		require.Equal(t, mockStorage.keyPEM, fakeNewKeyPEM)
	})

	t.Run("should store new server cert if expiry check fails", func(t *testing.T) {
		fakeExpiringCertPEM, fakeExpiringKeyPEM := []byte{1, 2, 3}, []byte{4, 5, 6}
		fakeNewCertPEM, fakeNewKeyPEM := []byte{7, 8, 9}, []byte{10, 11, 12}
		mockStorage := &mockServerCertStorage{
			certPEM: fakeExpiringCertPEM,
			keyPEM:  fakeExpiringKeyPEM,
		}
		sut := serverCertProviderImpl{
			storage:   mockStorage,
			checker:   &mockCertExpiryChecker{err: errors.New("failed")},
			generator: &mockServerCertGenerator{cert: fakeNewCertPEM, key: fakeNewKeyPEM},
		}

		_, _, err := sut.provideCert(context.Background(), serverCertConfig{})
		require.NoError(t, err)
		require.Equal(t, mockStorage.certPEM, fakeNewCertPEM)
		require.Equal(t, mockStorage.keyPEM, fakeNewKeyPEM)
	})

	t.Run("should return existing cert if not expired", func(t *testing.T) {
		fakeCertPEM, fakeKeyPEM := []byte{1, 2, 3}, []byte{4, 5, 6}
		mockStorage := &mockServerCertStorage{
			certPEM: fakeCertPEM,
			keyPEM:  fakeKeyPEM,
		}
		sut := serverCertProviderImpl{
			storage: mockStorage,
			checker: &mockCertExpiryChecker{certValid: true},
		}

		_, _, err := sut.provideCert(context.Background(), serverCertConfig{})
		require.NoError(t, err)
		require.Equal(t, mockStorage.certPEM, fakeCertPEM)
		require.Equal(t, mockStorage.keyPEM, fakeKeyPEM)
	})
}
