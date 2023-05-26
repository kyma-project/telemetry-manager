package webhookcert

import (
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"
)

const (
	rsaKeySize   = 2048
	duration365d = time.Hour * 24 * 365
	caCertMaxAge = duration365d
)

type caCertGeneratorImpl struct {
	clock clock
}

func (g *caCertGeneratorImpl) generateCert() ([]byte, []byte, error) {
	caCert, caKey, err := g.generateCertInternal()
	if err != nil {
		return nil, nil, err
	}

	caCertPEM, err := encodeCertPEM(caCert.Raw)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to pem encode cert: %w", err)
	}

	caKeyPEM, err := encodeKeyPEM(caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to pem encode key: %w", err)
	}

	return caCertPEM, caKeyPEM, nil
}

func (g *caCertGeneratorImpl) generateCertInternal() (*x509.Certificate, *rsa.PrivateKey, error) {
	caKey, err := rsa.GenerateKey(crand.Reader, rsaKeySize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generateCert rsa private key: %w", err)
	}

	validFrom := g.clock.now().Add(-time.Hour).UTC() // valid an hour earlier to avoid flakes due to clock skew
	validTo := validFrom.Add(caCertMaxAge).UTC()

	caCertTemplate := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			Organization: []string{"kyma-project.io"},
		},
		NotBefore:             validFrom,
		NotAfter:              validTo,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDERBytes, err := x509.CreateCertificate(crand.Reader, &caCertTemplate, &caCertTemplate, caKey.Public(), caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create x509 cert: %w", err)
	}

	caCert, err := x509.ParseCertificate(certDERBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse x509 cert: %w", err)
	}

	return caCert, caKey, nil
}
