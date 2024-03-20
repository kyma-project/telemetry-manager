package cert

import (
	_ "crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

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

	privateKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %v", err)
	}

	// Validate private key
	err = validatePrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("private key validation failed: %v", err)
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
