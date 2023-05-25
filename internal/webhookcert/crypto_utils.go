package webhookcert

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

const (
	pemBlockTypeCert = "CERTIFICATE"
	pemBlockTypeKey  = "RSA PRIVATE KEY"
)

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
