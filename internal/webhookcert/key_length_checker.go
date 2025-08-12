package webhookcert

import (
	"context"
	"fmt"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type keyLengthChecker interface {
	checkKeyLength(ctx context.Context, keyPEM []byte) (bool, error)
}

type keyLengthCheckerImpl struct {
	expectedKeySize int
}

// checkKeyLength checks if the provided PEM-encoded key has the desired length
func (c *keyLengthCheckerImpl) checkKeyLength(ctx context.Context, keyPEM []byte) (bool, error) {
	key, err := parseKeyPEM(keyPEM)
	if err != nil {
		return false, fmt.Errorf("failed to parse key PEM: %w", err)
	}

	if key.N.BitLen() != c.expectedKeySize {
		logf.FromContext(ctx).Info("CA key length check failed",
			"currentLength", key.N.BitLen(),
			"desiredLength", c.expectedKeySize)

		return false, nil
	}

	return true, nil
}
