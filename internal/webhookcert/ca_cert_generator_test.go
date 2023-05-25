package webhookcert

import (
	"crypto/rsa"
	"testing"

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
}
