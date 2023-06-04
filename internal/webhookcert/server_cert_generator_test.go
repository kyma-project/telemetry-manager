package webhookcert

import (
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateServerCert(t *testing.T) {
	caCertGen := caCertGeneratorImpl{clock: mockClock{}}
	sut := serverCertGeneratorImpl{clock: mockClock{}}
	t.Run("fails if nil input", func(t *testing.T) {
		_, _, err := sut.generateCert(serverCertConfig{host: "my-webhook.my-namespace"})
		require.Error(t, err)
	})

	t.Run("fails if invalid input", func(t *testing.T) {
		invalidCertPEM := []byte{1, 2, 3}
		invalidKeyPEM := []byte{1, 2, 3}
		_, _, err := sut.generateCert(serverCertConfig{host: "my-webhook.my-namespace", caCertPEM: invalidCertPEM, caKeyPEM: invalidKeyPEM})
		require.Error(t, err)
	})

	t.Run("succeeds", func(t *testing.T) {
		caCertPEM, caKeyPEM, err := caCertGen.generateCert()
		require.NoError(t, err)

		serverCertPEM, serverKeyPEM, err := sut.generateCert(serverCertConfig{host: "my-webhook.my-namespace", caCertPEM: caCertPEM, caKeyPEM: caKeyPEM})
		require.NoError(t, err)
		require.NotNil(t, serverCertPEM)
		require.NotNil(t, serverKeyPEM)
	})

	t.Run("generates valid x509 cert", func(t *testing.T) {
		caCertPEM, caKeyPEM, err := caCertGen.generateCert()
		require.NoError(t, err)

		serverCertPEM, _, err := sut.generateCert(serverCertConfig{host: "my-webhook.my-namespace", caCertPEM: caCertPEM, caKeyPEM: caKeyPEM})
		require.NoError(t, err)

		serverCert, err := parseCertPEM(serverCertPEM)
		require.NoError(t, err)
		require.NotNil(t, serverCert)
		require.True(t, strings.Contains(serverCert.Subject.CommonName, "my-webhook.my-namespace"))
	})

	t.Run("generates valid rsa private key", func(t *testing.T) {
		caCertPEM, caKeyPEM, err := caCertGen.generateCert()
		require.NoError(t, err)

		_, serverKeyPEM, err := sut.generateCert(serverCertConfig{host: "my-webhook.my-namespace", caCertPEM: caCertPEM, caKeyPEM: caKeyPEM})
		require.NoError(t, err)

		serverKey, err := parseKeyPEM(serverKeyPEM)
		require.NoError(t, err)
		require.NotNil(t, serverKey)
	})

	t.Run("generates matching cert and private key", func(t *testing.T) {
		caCertPEM, caKeyPEM, err := caCertGen.generateCert()
		require.NoError(t, err)

		serverCertPEM, serverKeyPEM, err := sut.generateCert(serverCertConfig{host: "my-webhook.my-namespace", caCertPEM: caCertPEM, caKeyPEM: caKeyPEM})
		require.NoError(t, err)

		serverCert, err := parseCertPEM(serverCertPEM)
		require.NoError(t, err)

		serverCertPublicKey, isRSA := serverCert.PublicKey.(*rsa.PublicKey)
		require.True(t, isRSA, "not an rsa public key")

		serverPrivateKey, err := parseKeyPEM(serverKeyPEM)
		require.NoError(t, err)

		require.Zero(t, serverCertPublicKey.N.Cmp(serverPrivateKey.N), "keys do not match")
	})

	t.Run("generates cert with alt DNS names", func(t *testing.T) {
		caCertPEM, caKeyPEM, err := caCertGen.generateCert()
		require.NoError(t, err)

		serverCertPEM, _, err := sut.generateCert(serverCertConfig{host: "my-webhook.my-namespace", alternativeDNSNames: []string{"foo", "bar"}, caCertPEM: caCertPEM, caKeyPEM: caKeyPEM})
		require.NoError(t, err)

		serverCert, err := parseCertPEM(serverCertPEM)
		require.NoError(t, err)
		require.NotNil(t, serverCert)
		require.Contains(t, serverCert.DNSNames, "my-webhook.my-namespace")
		require.Contains(t, serverCert.DNSNames, "foo")
		require.Contains(t, serverCert.DNSNames, "bar")
	})

	t.Run("generates cert that is valid since 1 hour", func(t *testing.T) {
		fakeNow := time.Date(2023, 5, 25, 12, 0, 0, 0, time.UTC)
		sut.clock = mockClock{t: fakeNow}

		caCertPEM, caKeyPEM, err := caCertGen.generateCert()
		require.NoError(t, err)

		serverCertPEM, _, err := sut.generateCert(serverCertConfig{host: "my-webhook.my-namespace", caCertPEM: caCertPEM, caKeyPEM: caKeyPEM})
		require.NoError(t, err)

		serverCert, err := parseCertPEM(serverCertPEM)
		require.NoError(t, err)

		require.Equal(t, serverCert.NotBefore, time.Date(2023, 5, 25, 11, 0, 0, 0, time.UTC))
	})

	t.Run("generates cert that expires in 1 week - 1 hour", func(t *testing.T) {
		fakeNow := time.Date(2023, 5, 25, 12, 0, 0, 0, time.UTC)
		sut.clock = mockClock{t: fakeNow}

		caCertPEM, caKeyPEM, err := caCertGen.generateCert()
		require.NoError(t, err)

		serverCertPEM, _, err := sut.generateCert(serverCertConfig{host: "my-webhook.my-namespace", caCertPEM: caCertPEM, caKeyPEM: caKeyPEM})
		require.NoError(t, err)

		serverCert, err := parseCertPEM(serverCertPEM)
		require.NoError(t, err)

		require.Equal(t, serverCert.NotAfter, time.Date(2023, 6, 1, 11, 0, 0, 0, time.UTC))
	})
}
