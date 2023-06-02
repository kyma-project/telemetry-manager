package webhookcert

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	certDir := "testdata"
	require.NoError(t, os.Mkdir(certDir, 0750))
	sut := serverCertStorageImpl{certDir: certDir}
	defer func() {
		require.NoError(t, os.RemoveAll(certDir))
	}()

	fakeCertPEM, fakeKeyPEM := []byte{1, 2, 3}, []byte{4, 5, 6}
	err := sut.save(fakeCertPEM, fakeKeyPEM)
	require.NoError(t, err)

	certPEM, keyPEM, err := sut.load()
	require.NoError(t, err)
	require.Equal(t, fakeCertPEM, certPEM)
	require.Equal(t, fakeKeyPEM, keyPEM)
}
