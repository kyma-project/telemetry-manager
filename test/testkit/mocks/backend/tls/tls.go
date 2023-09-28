package tls

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

type Certs struct {
	CaCertPem     bytes.Buffer
	ServerCertPem bytes.Buffer
	ServerKeyPem  bytes.Buffer
	ClientCertPem bytes.Buffer
	ClientKeyPem  bytes.Buffer
}

// helper function to create a cert template with a serial number and other required fields
func certTemplate(serialNumber int64) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber:          big.NewInt(serialNumber),
		Subject:               pkix.Name{Organization: []string{"Kyma E2E Test"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
	}
}

func GenerateTLSCerts(serverDNSName string) (Certs, error) {
	var certs Certs

	// CA Certificate
	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return certs, err
	}

	caTemplate := certTemplate(1)
	caTemplate.IsCA = true
	caTemplate.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	caTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}

	caCertBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caPrivateKey.Public(), caPrivateKey)
	if err != nil {
		return certs, err
	}

	err = pem.Encode(&certs.CaCertPem, &pem.Block{Type: "CERTIFICATE", Bytes: caCertBytes})
	if err != nil {
		return certs, err
	}

	// Server Certificate (signed by CA certificate)
	serverPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return certs, err
	}

	err = pem.Encode(&certs.ServerKeyPem, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverPrivateKey)})
	if err != nil {
		return certs, err
	}

	serverTemplate := certTemplate(2)
	serverTemplate.KeyUsage = x509.KeyUsageDigitalSignature
	serverTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	serverTemplate.DNSNames = []string{serverDNSName}

	serverBytes, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, serverPrivateKey.Public(), caPrivateKey)
	if err != nil {
		return certs, err
	}

	err = pem.Encode(&certs.ServerCertPem, &pem.Block{Type: "CERTIFICATE", Bytes: serverBytes})
	if err != nil {
		return certs, err
	}

	// Client Certificate (signed by CA certificate)
	clientPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return certs, err
	}

	err = pem.Encode(&certs.ClientKeyPem, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientPrivateKey)})
	if err != nil {
		return certs, err
	}

	clientTemplate := certTemplate(3)
	serverTemplate.KeyUsage = x509.KeyUsageDigitalSignature
	serverTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	clientCertBytes, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, clientPrivateKey.Public(), caPrivateKey)
	if err != nil {
		return certs, err
	}

	err = pem.Encode(&certs.ClientCertPem, &pem.Block{Type: "CERTIFICATE", Bytes: clientCertBytes})
	if err != nil {
		return certs, err
	}

	return certs, nil
}
