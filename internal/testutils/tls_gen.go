package testutils

import (
	"bytes"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

type CertBuilder struct {
	serverDNSName   string
	clientNotBefore time.Time
	clientNotAfter  time.Time
	clientInvalid   bool
	commonName      string
}

func NewCertBuilder(serverName, serverNamespace string) *CertBuilder {
	return &CertBuilder{
		serverDNSName:   serverName + "." + serverNamespace + ".svc.cluster.local",
		clientNotBefore: time.Now(),
		clientNotAfter:  time.Now().AddDate(0, 0, 30),
		commonName:      "default.com",
	}
}

type ServerCerts struct {
	CaCertPem bytes.Buffer

	ServerCertPem bytes.Buffer
	ServerKeyPem  bytes.Buffer
}

type ClientCerts struct {
	CaCertPem bytes.Buffer

	ClientCertPem bytes.Buffer
	ClientKeyPem  bytes.Buffer
}

func (c *CertBuilder) WithExpiredClientCert() *CertBuilder {
	c.clientNotAfter = time.Now().AddDate(0, 0, -7)
	return c
}

func (c *CertBuilder) WithAboutToExpireClientCert() *CertBuilder {
	c.clientNotAfter = time.Now().AddDate(0, 0, 7)
	return c
}

func (c *CertBuilder) WithInvalidClientCert() *CertBuilder {
	return &CertBuilder{clientInvalid: true}
}

func (c *CertBuilder) WithCommonName(commonName string) *CertBuilder {
	c.commonName = commonName
	return c
}

func (c *CertBuilder) Build() (*ServerCerts, *ClientCerts, error) {
	// CA Certificate
	caPrivateKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	caTemplate := c.caCertTemplate()
	caTemplate.IsCA = true
	caTemplate.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	caTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}

	caCertBytes, err := x509.CreateCertificate(crand.Reader, caTemplate, caTemplate, caPrivateKey.Public(), caPrivateKey)
	if err != nil {
		return nil, nil, err
	}

	caCertPem := bytes.Buffer{}
	err = pem.Encode(&caCertPem, &pem.Block{Type: "CERTIFICATE", Bytes: caCertBytes})
	if err != nil {
		return nil, nil, err
	}

	// Server Certificate (signed by CA certificate)
	serverPrivateKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serverKeyPem := bytes.Buffer{}
	err = pem.Encode(&serverKeyPem, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverPrivateKey)})
	if err != nil {
		return nil, nil, err
	}

	serverTemplate := c.serverCertTemplate()
	serverTemplate.KeyUsage = x509.KeyUsageDigitalSignature
	serverTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	serverTemplate.DNSNames = []string{c.serverDNSName}

	serverBytes, err := x509.CreateCertificate(crand.Reader, serverTemplate, caTemplate, serverPrivateKey.Public(), caPrivateKey)
	if err != nil {
		return nil, nil, err
	}

	serverCertPem := bytes.Buffer{}
	err = pem.Encode(&serverCertPem, &pem.Block{Type: "CERTIFICATE", Bytes: serverBytes})
	if err != nil {
		return nil, nil, err
	}

	// Client Certificate (signed by CA certificate)
	clientPrivateKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	clientKeyPem := bytes.Buffer{}
	err = pem.Encode(&clientKeyPem, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientPrivateKey)})
	if err != nil {
		return nil, nil, err
	}

	clientTemplate := c.clientCertTemplate()
	serverTemplate.KeyUsage = x509.KeyUsageDigitalSignature
	serverTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	clientCertBytes, err := x509.CreateCertificate(crand.Reader, clientTemplate, caTemplate, clientPrivateKey.Public(), caPrivateKey)
	if err != nil {
		return nil, nil, err
	}

	clientCertPem := bytes.Buffer{}
	if !c.clientInvalid {
		err = pem.Encode(&clientCertPem, &pem.Block{Type: "CERTIFICATE", Bytes: clientCertBytes})
		if err != nil {
			return nil, nil, err
		}
	} else {
		clientCertPem.WriteString("invalid")
	}

	return &ServerCerts{
			CaCertPem:     caCertPem,
			ServerCertPem: serverCertPem,
			ServerKeyPem:  serverKeyPem,
		}, &ClientCerts{
			CaCertPem:     caCertPem,
			ClientCertPem: clientCertPem,
			ClientKeyPem:  clientKeyPem,
		}, nil
}

func (c *CertBuilder) caCertTemplate() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: fmt.Sprintf("ca-%s", c.commonName)},
		NotBefore:             c.clientNotBefore,
		NotAfter:              c.clientNotAfter,
		BasicConstraintsValid: true,
	}
}

func (c *CertBuilder) clientCertTemplate() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: fmt.Sprintf("client-%s", c.commonName)},
		NotBefore:             c.clientNotBefore,
		NotAfter:              c.clientNotAfter,
		BasicConstraintsValid: true,
	}
}

func (c *CertBuilder) serverCertTemplate() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		Subject:               pkix.Name{CommonName: fmt.Sprintf("server-%s", c.commonName)},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
	}
}

func BuildInvalidKeyPair(caCertPem, clientCertPem, nonMatchingClientKeyPem bytes.Buffer) *ClientCerts {
	return &ClientCerts{
		CaCertPem:     caCertPem,
		ClientCertPem: clientCertPem,
		ClientKeyPem:  nonMatchingClientKeyPem,
	}
}
