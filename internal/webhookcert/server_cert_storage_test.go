package webhookcert

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	certDir := t.TempDir()

	sut := serverCertStorageImpl{certDir: certDir}

	fakeCertPEM, fakeKeyPEM := []byte{1, 2, 3}, []byte{4, 5, 6}
	require.NoError(t, sut.save(fakeCertPEM, fakeKeyPEM))

	certPEM, keyPEM, err := sut.load()
	require.NoError(t, err)
	require.Equal(t, fakeCertPEM, certPEM)
	require.Equal(t, fakeKeyPEM, keyPEM)
}
