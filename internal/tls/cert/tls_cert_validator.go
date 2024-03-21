package cert

import (
	_ "crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

type TLSCertValidator struct {
}

type TLSCertValidationResult struct {
	CertPEMBlockValid       bool
	CertValid               bool
	PrivateKeyPEMBlockValid bool
	PrivateKeyValid         bool
	Validity                time.Time
}

func (tcv *TLSCertValidator) ValidateCertificate(certPEM []byte, keyPEM []byte) TLSCertValidationResult {
	result := TLSCertValidationResult{}

	// Parse the certificate
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		result.CertPEMBlockValid = false
	}
	_, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		result.CertValid = false
	}

	// Parse the private key
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil || keyBlock.Type != "PRIVATE KEY" {
		result.PrivateKeyPEMBlockValid = false
	}

	_, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		result.PrivateKeyValid = false
	}
	return result
}

func ValidateCertificate(certPEM []byte, keyPEM []byte) error {

	// Parse the certificate
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("failed to decode PEM block containing certificate")
	}
	_, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate %v", err)
	}

	// Parse the private key
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil || keyBlock.Type != "PRIVATE KEY" {
		return fmt.Errorf("failed to decode PEM block containing private key")
	}

	_, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %v", err)
	}

	return nil
}

func CheckExpiryDate(certPEM []byte, checkTime time.Time) (bool, error) {

	// Parse the certificate
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return false, fmt.Errorf("failed to decode PEM block containing certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("failed to parse certificate %v", err)
	}

	return checkTime.Before(cert.NotAfter), nil
}

func validatePrivateKey(key *rsa.PrivateKey) error {
	err := key.Validate()
	if err != nil {
		return fmt.Errorf("private key validation failed: %v", err)
	}

	return nil
}
