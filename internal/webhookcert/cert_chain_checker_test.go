package webhookcert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckChain(t *testing.T) {
	caCert, caKey := generateCACertKey(time.Now())
	serverCert := generateServerCert(caCert, caKey, time.Now())

	tests := []struct {
		summary       string
		serverCertPEM []byte
		caCertPEM     []byte
		expectValid   bool
		expectError   bool
	}{
		{
			summary:     "nil server cert",
			caCertPEM:   caCert,
			expectError: true,
		},
		{
			summary:       "nil ca cert",
			serverCertPEM: serverCert,
			expectError:   true,
		},
		{
			summary:       "invalid server cert",
			caCertPEM:     caCert,
			serverCertPEM: []byte{1, 2, 3},
			expectError:   true,
		},
		{
			summary:       "invalid ca cert",
			caCertPEM:     []byte{1, 2, 3},
			serverCertPEM: serverCert,
			expectError:   true,
		},
		{
			summary:       "ca is not root",
			caCertPEM:     generateCACert(time.Now()),
			serverCertPEM: serverCert,
			expectError:   true,
		},
		{
			summary:       "ca is root",
			caCertPEM:     caCert,
			serverCertPEM: serverCert,
			expectValid:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.summary, func(t *testing.T) {
			sut := certChainCheckerImpl{}
			valid, err := sut.checkRoot(t.Context(), tc.serverCertPEM, tc.caCertPEM)

			if tc.expectError {
				require.Error(t, err, "Expected error but got nil")
			} else {
				require.NoError(t, err, "Did not expect error but got one: %s", err.Error())
			}

			require.Equal(t, tc.expectValid, valid, "Expected valid to be %v but got %v", tc.expectValid, valid)
		})
	}
}
