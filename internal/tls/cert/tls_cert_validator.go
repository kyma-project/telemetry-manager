package cert

import (
	_ "crypto"
	"crypto/x509"
	"encoding/pem"
	"time"
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
	block, _ := pem.Decode(certPEM)
	if block == nil {
		result.CertValid = false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		result.CertValid = false
	}

	// Parse the private key
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		result.PrivateKeyValid = false
	}

	_, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		result.PrivateKeyValid = false
	}

	if result.CertValid && result.PrivateKeyValid {
		result.Validity = cert.NotAfter
	}

	return result
}
