package cert

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	_ "crypto"
)

type TLSCertValidator struct {
}

type TLSCertValidationResult struct {
	CertValid                   bool
	CertValidationMessage       string
	PrivateKeyValid             bool
	PrivateKeyValidationMessage string
	Validity                    time.Time
}

func (tcv *TLSCertValidator) ValidateCertificate(certPEM []byte, keyPEM []byte) TLSCertValidationResult {
	result := TLSCertValidationResult{
		CertValid:       true,
		PrivateKeyValid: true,
		Validity:        time.Now().Add(time.Hour * 24 * 365),
	}

	// Make a best effort replacement of linebreaks in cert/key if present.
	certReplaced := []byte(strings.ReplaceAll(string(certPEM), "\\n", "\n"))
	keyReplaced := []byte(strings.ReplaceAll(string(keyPEM), "\\n", "\n"))

	// Parse the certificate
	cert, err := parseCertificate(certReplaced)
	if err != nil {
		result.CertValid = false
		result.CertValidationMessage = err.Error()
	}

	// Parse the private key
	if _, err := parsePrivateKey(keyReplaced); err != nil {
		result.PrivateKeyValid = false
		result.PrivateKeyValidationMessage = err.Error()
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
