package tlscert

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/secretref"

	_ "crypto"
)

type TLSCertValidator struct {
	client client.Reader
}

type TLSCertValidationResult struct {
	CertValid                   bool
	CertValidationMessage       string
	PrivateKeyValid             bool
	PrivateKeyValidationMessage string
	Validity                    time.Time
}

func New(client client.Client) *TLSCertValidator {
	return &TLSCertValidator{
		client: client,
	}
}

func (tcv *TLSCertValidator) ValidateCertificate(ctx context.Context, certPEM *telemetryv1alpha1.ValueType, keyPEM *telemetryv1alpha1.ValueType) TLSCertValidationResult {
	result := TLSCertValidationResult{
		CertValid:       true,
		PrivateKeyValid: true,
		Validity:        time.Now().Add(time.Hour * 24 * 365),
	}

	certData, err := resolveValue(ctx, tcv.client, *certPEM)
	if err != nil {
		result.CertValid = false
		return result
	}

	keyData, err := resolveValue(ctx, tcv.client, *keyPEM)
	if err != nil {
		result.PrivateKeyValid = false
		return result
	}

	// Make a best effort replacement of linebreaks in cert/key if present.
	sanitizedCert := bytes.ReplaceAll(certData, []byte("\\n"), []byte("\n"))
	sanitizedKey := bytes.ReplaceAll(keyData, []byte("\\n"), []byte("\n"))

	// Parse the certificate
	cert, err := parseCertificate(sanitizedCert)
	if err != nil {
		result.CertValid = false
		result.CertValidationMessage = err.Error()
	}

	// Parse the private key
	if _, err := parsePrivateKey(sanitizedKey); err != nil {
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

func resolveValue(ctx context.Context, c client.Reader, value telemetryv1alpha1.ValueType) ([]byte, error) {
	if value.Value != "" {
		return []byte(value.Value), nil
	}
	if value.ValueFrom.IsSecretKeyRef() {
		return secretref.GetValue(ctx, c, *value.ValueFrom.SecretKeyRef)
	}

	return nil, fmt.Errorf("either value or secret key reference must be defined")
}
