package webhookcert

import (
	"crypto/rsa"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateCACert(t *testing.T) {
	sut := caCertGeneratorImpl{
		clock: mockClock{},
	}

	t.Run("succeeds", func(t *testing.T) {
		caCertPEM, caKeyPEM, err := sut.generateCert()
		require.NoError(t, err)
		require.NotEmpty(t, caCertPEM)
		require.NotEmpty(t, caKeyPEM)
	})

	t.Run("generates valid x509 cert", func(t *testing.T) {
		caCertPEM, _, err := sut.generateCert()
		require.NoError(t, err)

		caCert, err := parseCertPEM(caCertPEM)
		require.NoError(t, err)
		require.NotNil(t, caCert)
		require.NotNil(t, caCert.Subject.Organization)
	})

	t.Run("generates valid rsa private key", func(t *testing.T) {
		_, caKeyPEM, err := sut.generateCert()
		require.NoError(t, err)

		caKey, err := parseKeyPEM(caKeyPEM)
		require.NoError(t, err)
		require.NotNil(t, caKey)
	})

	t.Run("generates matching cert and private key", func(t *testing.T) {
		caCertPEM, caKeyPEM, err := sut.generateCert()
		require.NoError(t, err)

		caCert, err := parseCertPEM(caCertPEM)
		require.NoError(t, err)

		caCertPublicKey, isRSA := caCert.PublicKey.(*rsa.PublicKey)
		require.True(t, isRSA, "not an rsa public key")

		caPrivateKey, err := parseKeyPEM(caKeyPEM)
		require.NoError(t, err)

		require.Zero(t, caCertPublicKey.N.Cmp(caPrivateKey.N), "keys do not match")
	})

	t.Run("generates cert that is valid since 1 hour", func(t *testing.T) {
		fakeNow := time.Date(2023, 5, 25, 12, 0, 0, 0, time.UTC)
		sut.clock = mockClock{t: fakeNow}

		caCertPEM, _, err := sut.generateCert()
		require.NoError(t, err)

		caCert, err := parseCertPEM(caCertPEM)
		require.NoError(t, err)

		require.Equal(t, caCert.NotBefore, time.Date(2023, 5, 25, 11, 0, 0, 0, time.UTC))
	})

	t.Run("generates cert that expires in 1 year - 1 hour", func(t *testing.T) {
		fakeNow := time.Date(2023, 5, 25, 12, 0, 0, 0, time.UTC)
		sut.clock = mockClock{t: fakeNow}

		caCertPEM, _, err := sut.generateCert()
		require.NoError(t, err)

		caCert, err := parseCertPEM(caCertPEM)
		require.NoError(t, err)

		require.Equal(t, caCert.NotAfter, time.Date(2024, 5, 24, 11, 0, 0, 0, time.UTC))
	})
}
