package webhookcert

import (
	"crypto/x509"
	"errors"
	"fmt"
)

type certChainChecker interface {
	checkRoot(serverCertPEM []byte, caCertPEM []byte) (bool, error)
}

type certChainCheckerImpl struct {
}

func (c *certChainCheckerImpl) checkRoot(serverCertPEM []byte, caCertPEM []byte) (bool, error) {
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(caCertPEM)
	if !ok {
		return false, errors.New("failed to parse root certificate")
	}

	serverCert, err := parseCertPEM(serverCertPEM)
	if err != nil {
		return false, fmt.Errorf("failed to parse x509 cert: %w", err)
	}

	chains, err := serverCert.Verify(x509.VerifyOptions{Roots: roots})
	if err != nil {
		return false, fmt.Errorf("failed to verify x509 cert: %w", err)
	}

	return len(chains) > 0, nil
}
