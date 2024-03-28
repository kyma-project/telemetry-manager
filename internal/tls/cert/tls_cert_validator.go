package cert

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	_ "crypto"
)

type TLSCertValidator struct {
}

type TLSCertValidationResult struct {
	CertValid       bool
	PrivateKeyValid bool
	Validity        time.Time
}

func (tcv *TLSCertValidator) ValidateCertificate(certPEM []byte, keyPEM []byte) TLSCertValidationResult {
	result := TLSCertValidationResult{
		CertValid:       true,
		PrivateKeyValid: true,
		Validity:        time.Now().Add(time.Hour * 24 * 365),
	}

	// Parse the certificate
	cert, err := parseCertificate(certPEM)
	if err != nil {
		result.CertValid = false
	}

	// Parse the private key
	if _, err := parsePrivateKey(keyPEM); err != nil {
		result.PrivateKeyValid = false
	}

	if result.CertValid && result.PrivateKeyValid {
		result.Validity = cert.NotAfter
	}

	return result
}

func parseCertificate(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parsePrivateKey(keyPEM []byte) (interface{}, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}
	// try to parse as PKCS8 / PRIVATE KEY
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)

	// try to parse as PKCS1 / RSA PRIVATE KEY
	if err != nil {
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}

	return privateKey, nil
}
