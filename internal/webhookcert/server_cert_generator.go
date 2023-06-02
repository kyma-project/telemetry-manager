package webhookcert

import (
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math"
	"math/big"
	"time"
)

const (
	duration1w       = time.Hour * 24 * 7
	serverCertMaxAge = duration1w
)

type serverCertGeneratorImpl struct {
	clock clock
}

func (g *serverCertGeneratorImpl) generateCert(config serverCertConfig) ([]byte, []byte, error) {
	caCert, err := parseCertPEM(config.caCertPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse ca cert: %w", err)
	}

	caKey, err := parseKeyPEM(config.caKeyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse ca key: %w", err)
	}

	return g.generateCertInternal(config.host, config.alternativeDNSNames, caCert, caKey)
}

func (g *serverCertGeneratorImpl) generateCertInternal(host string, alternativeDNSNames []string, caCert *x509.Certificate, caKey *rsa.PrivateKey) ([]byte, []byte, error) {
	// returns a uniform random value in [0, max-1), then add 1 to serial to make it a uniform random value in [1, max).
	serial, err := crand.Int(crand.Reader, new(big.Int).SetInt64(math.MaxInt64-1))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generateCert serial number: %w", err)
	}

	validFrom := g.clock.now().Add(-time.Hour).UTC() // valid an hour earlier to avoid flakes due to clock skew
	validTo := validFrom.Add(serverCertMaxAge).UTC()

	certTemplate := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("%s@%d", host, time.Now().Unix()),
		},
		NotBefore:             validFrom,
		NotAfter:              validTo,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certTemplate.DNSNames = append(certTemplate.DNSNames, host)
	certTemplate.DNSNames = append(certTemplate.DNSNames, alternativeDNSNames...)

	key, err := rsa.GenerateKey(crand.Reader, rsaKeySize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generateCert rsa private key: %w", err)
	}

	certBytes, err := x509.CreateCertificate(crand.Reader, &certTemplate, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create x509 cert: %w", err)
	}

	certPEM, err := encodeCertPEM(certBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to pem encode cert: %w", err)
	}

	keyPEM, err := encodeKeyPEM(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to pem encode key: %w", err)
	}

	return certPEM, keyPEM, nil
}
