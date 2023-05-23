package webhookcert

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateCACertKey(t *testing.T) {
	t.Run("succeeds", func(t *testing.T) {
		caCertPEM, caKeyPEM, err := generateCACertKey()
		require.NoError(t, err)
		require.NotEmpty(t, caCertPEM)
		require.NotEmpty(t, caKeyPEM)
	})

	t.Run("generates valid x509 cert", func(t *testing.T) {
		caCertPEM, _, err := generateCACertKey()
		require.NoError(t, err)

		caCert, err := parseCertPEM(caCertPEM)
		require.NoError(t, err)
		require.NotNil(t, caCert)
	})

	t.Run("generates valid rsa private key", func(t *testing.T) {
		_, caKeyPEM, err := generateCACertKey()
		require.NoError(t, err)

		caKey, err := parseKeyPEM(caKeyPEM)
		require.NoError(t, err)
		require.NotNil(t, caKey)
	})
}
