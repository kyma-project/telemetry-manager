package webhookcert

import (
	"context"
	crand "crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckKeyLength(t *testing.T) {
	tt := []struct {
		name     string
		key      []byte
		expected bool
	}{
		{
			name:     "key with desired length",
			key:      generateKey(t, 4096),
			expected: true,
		}, {
			name:     "key with undesired length",
			key:      generateKey(t, 2048),
			expected: false,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			checker := &caKeyLengthCheckerImpl{}
			result, err := checker.checkKeyLength(context.TODO(), tc.key)
			require.NoError(t, err, "failed to check key length")
			require.Equal(t, tc.expected, result)
		})
	}
}

func generateKey(t *testing.T, keyLen int) []byte {
	key, err := rsa.GenerateKey(crand.Reader, keyLen)
	require.NoError(t, err, "failed to generate key")
	encodedKey, err := encodeKeyPEM(key)
	require.NoError(t, err, "failed to encode key")
	return encodedKey
}
