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
}

// checkKeyLength checks if the provided PEM-encoded key has the desired length
func (c *keyLengthCheckerImpl) checkKeyLength(ctx context.Context, keyPEM []byte) (bool, error) {
	key, err := parseKeyPEM(keyPEM)
	if err != nil {
		return false, fmt.Errorf("failed to parse key PEM: %w", err)
	}

	if key.N.BitLen() != rsaKeySize {
		logf.FromContext(ctx).Info("CA key length check failed",
			"currentLength", key.N.BitLen(),
			"desiredLength", rsaKeySize)

		return false, nil
	}

	return true, nil
}
