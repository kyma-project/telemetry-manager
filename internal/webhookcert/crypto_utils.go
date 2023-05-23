package webhookcert

import (
	"bytes"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"
)

const (
	rsaKeySize       = 2048
	pemBlockTypeCert = "CERTIFICATE"
	pemBlockTypeKey  = "RSA PRIVATE KEY"

	duration365d     = time.Hour * 24 * 365
	duration1w       = time.Hour * 24 * 7
	caCertMaxAge     = duration365d
	serverCertMaxAge = duration1w
)

func generateCACertKey() ([]byte, []byte, error) {
	caCert, caKey, err := generateCACertKeyInternal()
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

func generateCACertKeyInternal() (*x509.Certificate, *rsa.PrivateKey, error) {
	caKey, err := rsa.GenerateKey(crand.Reader, rsaKeySize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate rsa private key: %w", err)
	}

	validFrom := time.Now().Add(-time.Hour).UTC()
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

func generateServerCertKey(host string, alternativeDNSNames []string, caCertPEM, caKeyPEM []byte) ([]byte, []byte, error) {
	caCert, err := parseCertPEM(caCertPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse ca cert: %w", err)
	}

	caKey, err := parseKeyPEM(caKeyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse ca key: %w", err)
	}

	return generateServerCertKeyInternal(host, alternativeDNSNames, caCert, caKey)
}

func generateServerCertKeyInternal(host string, alternativeDNSNames []string, caCert *x509.Certificate, caKey *rsa.PrivateKey) ([]byte, []byte, error) {
	// returns a uniform random value in [0, max-1), then add 1 to serial to make it a uniform random value in [1, max).
	serial, err := crand.Int(crand.Reader, new(big.Int).SetInt64(math.MaxInt64-1))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	validFrom := time.Now().Add(-time.Hour).UTC()
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
		return nil, nil, fmt.Errorf("failed to generate rsa private key: %w", err)
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

func encodeCertPEM(certDER []byte) ([]byte, error) {
	buffer := bytes.Buffer{}
	if err := pem.Encode(&buffer, &pem.Block{Type: pemBlockTypeCert, Bytes: certDER}); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func encodeKeyPEM(key *rsa.PrivateKey) ([]byte, error) {
	buffer := bytes.Buffer{}
	if err := pem.Encode(&buffer, &pem.Block{Type: pemBlockTypeKey, Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func parseCertPEM(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != pemBlockTypeCert {
		return nil, errors.New("not a cert")
	}

	return x509.ParseCertificate(block.Bytes)
}

func parseKeyPEM(keyPEM []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil || block.Type != pemBlockTypeKey {
		return nil, errors.New("not a private key")
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
