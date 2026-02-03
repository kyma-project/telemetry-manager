package test

import (
	"bytes"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

type CertBuilder struct {
	serverDNSName   string
	clientNotBefore time.Time
	clientNotAfter  time.Time
	clientInvalid   bool
	caInvalid       bool
}

func NewCertBuilder(serverName, serverNamespace string) *CertBuilder {
	return &CertBuilder{
		serverDNSName:   serverName + "." + serverNamespace + ".svc.cluster.local",
		clientNotBefore: time.Now(),
		clientNotAfter:  time.Now().AddDate(0, 0, 30),
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

// WithExpiredClientCert sets the client certificate to be already expired 7 days ago.
func (c *CertBuilder) WithExpiredClientCert() *CertBuilder {
	c.clientNotAfter = time.Now().AddDate(0, 0, -7)
	return c
}

// WithAboutToExpireClientCert sets the client certificate to be about to expire in 7 days.
func (c *CertBuilder) WithAboutToExpireClientCert() *CertBuilder {
	c.clientNotAfter = time.Now().AddDate(0, 0, 7)
	return c
}

// WithAboutToExpireShortlyClientCert sets the client certificate to be about to expire in 30 seconds.
func (c *CertBuilder) WithAboutToExpireShortlyClientCert() *CertBuilder {
	c.clientNotAfter = time.Now().Add(30 * time.Second)
	return c
}

func (c *CertBuilder) WithInvalidClientCert() *CertBuilder {
	return &CertBuilder{clientInvalid: true}
}

func (c *CertBuilder) WithInvalidCA() *CertBuilder {
	return &CertBuilder{caInvalid: true}
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
	if !c.caInvalid {
		err = pem.Encode(&caCertPem, &pem.Block{Type: "CERTIFICATE", Bytes: caCertBytes})
		if err != nil {
			return nil, nil, err
		}
	} else {
		caCertPem.WriteString("invalid")
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
		Subject:               pkix.Name{CommonName: "ca"},
		NotBefore:             c.clientNotBefore,
		NotAfter:              c.clientNotAfter,
		BasicConstraintsValid: true,
	}
}

func (c *CertBuilder) clientCertTemplate() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "client.com"},
		NotBefore:             c.clientNotBefore,
		NotAfter:              c.clientNotAfter,
		BasicConstraintsValid: true,
	}
}

func (c *CertBuilder) serverCertTemplate() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		Subject:               pkix.Name{CommonName: "server.com"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
	}
}
