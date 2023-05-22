package certgen

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

	"k8s.io/client-go/util/cert"
)

func generateCACertKey() ([]byte, []byte, error) {
	key, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %v", err)
	}
	cfg := cert.Config{
		CommonName: "kubernetes",
	}

	caCert, err := cert.NewSelfSignedCACert(cfg, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate self signed cert: %v", err)
	}

	certBuffer := bytes.Buffer{}
	if err := pem.Encode(&certBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw}); err != nil {
		return nil, nil, err
	}

	keyBuffer := bytes.Buffer{}
	if err := pem.Encode(&keyBuffer, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return nil, nil, err
	}

	return certBuffer.Bytes(), keyBuffer.Bytes(), nil
}

func generateServerCertKey(serviceName, namespace string, caCertPEM, caKeyPEM []byte) ([]byte, []byte, error) {
	cn := fmt.Sprintf("%s.%s.svc", serviceName, namespace)
	names := []string{
		serviceName,
		fmt.Sprintf("%s.%s", serviceName, namespace),
		fmt.Sprintf("%s.cluster.local", cn),
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock.Type != "CERTIFICATE" {
		return nil, nil, errors.New("not a cert")
	}

	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock.Type != "RSA PRIVATE KEY" {
		return nil, nil, errors.New("not a private key")
	}

	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	return generateCertSignedByCA(cn, names, caCert, caKey)
}

func generateCertSignedByCA(host string, alternateDNS []string, caCert *x509.Certificate, caKey *rsa.PrivateKey) ([]byte, []byte, error) {
	validFrom := time.Now().Add(-time.Hour) // valid an hour earlier to avoid flakes due to clock skew
	maxAge := time.Hour * 24 * 365          // one year self-signed certs
	// returns a uniform random value in [0, max-1), then add 1 to serial to make it a uniform random value in [1, max).
	serial, err := crand.Int(crand.Reader, new(big.Int).SetInt64(math.MaxInt64-1))
	if err != nil {
		return nil, nil, err
	}

	certTemplate := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("%s@%d", host, time.Now().Unix()),
		},
		NotBefore: validFrom,
		NotAfter:  validFrom.Add(maxAge),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certTemplate.DNSNames = append(certTemplate.DNSNames, host)
	certTemplate.DNSNames = append(certTemplate.DNSNames, alternateDNS...)

	key, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(crand.Reader, &certTemplate, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	certBuffer := bytes.Buffer{}
	if err := pem.Encode(&certBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return nil, nil, err
	}

	keyBuffer := bytes.Buffer{}
	if err := pem.Encode(&keyBuffer, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return nil, nil, err
	}

	return certBuffer.Bytes(), keyBuffer.Bytes(), nil
}
