package tlscert

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/secretref"

	_ "crypto"
)

var (
	ErrCertDecodeFailed = errors.New("failed to decode PEM block containing cert")
	ErrCertParseFailed  = errors.New("failed to parse certificate")

	ErrKeyDecodeFailed = errors.New("failed to decode PEM block containing private key")
	ErrKeyParseFailed  = errors.New("failed to parse private key")

	ErrValueResolveFailed = errors.New("failed to resolve value")

	ErrInvalidCertificateKeyPair = errors.New("certificate and private key do not match")
)

const twoWeeks = time.Hour * 24 * 7 * 2

type CertExpiredError struct {
	Expiry time.Time
}

func (cee *CertExpiredError) Error() string {
	return fmt.Sprintf("cert expired on %s", cee.Expiry)
}

func IsCertExpiredError(err error) bool {
	var errCertExpired *CertExpiredError
	return errors.As(err, &errCertExpired)
}

type CertAboutToExpireError struct {
	Expiry time.Time
}

func (cate *CertAboutToExpireError) Error() string {
	return fmt.Sprintf("cert is about to expire, it is valid until %s", cate.Expiry)
}

func IsCertAboutToExpireError(err error) bool {
	var errCertAboutToExpire *CertAboutToExpireError
	return errors.As(err, &errCertAboutToExpire)
}

type Validator struct {
	client client.Reader
	now    func() time.Time
}

func New(client client.Client) *Validator {
	return &Validator{
		client: client,
		now:    time.Now,
	}
}

func (v *Validator) ValidateCertificate(ctx context.Context, cert, key *telemetryv1alpha1.ValueType) error {
	certPEM, err := resolveValue(ctx, v.client, *cert)
	if err != nil {
		return err
	}

	keyPEM, err := resolveValue(ctx, v.client, *key)
	if err != nil {
		return err
	}

	// Make the best effort replacement of linebreaks in cert/key if present.
	sanitizedCert := bytes.ReplaceAll(certPEM, []byte("\\n"), []byte("\n"))
	sanitizedKey := bytes.ReplaceAll(keyPEM, []byte("\\n"), []byte("\n"))

	// Parse the certificate
	certExpiry, err := parseCertificate(sanitizedCert)
	if err != nil {
		return err
	}

	// Parse the private key
	if err = parsePrivateKey(sanitizedKey); err != nil {
		return err
	}

	_, err = tls.X509KeyPair(sanitizedCert, sanitizedKey)
	if err != nil {
		return ErrInvalidCertificateKeyPair
	}

	if v.now().After(certExpiry) {
		return &CertExpiredError{Expiry: certExpiry}
	}

	if certExpiry.Sub(v.now()) <= twoWeeks {
		return &CertAboutToExpireError{Expiry: certExpiry}
	}

	return nil
}

func parseCertificate(certPEM []byte) (time.Time, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return time.Time{}, ErrCertDecodeFailed
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, ErrCertParseFailed
	}

	return cert.NotAfter, nil
}

func parsePrivateKey(keyPEM []byte) error {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return ErrKeyDecodeFailed
	}

	_, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		if _, err = x509.ParsePKCS1PrivateKey(block.Bytes); err != nil {
			return ErrKeyParseFailed
		}
	}

	return nil
}

func resolveValue(ctx context.Context, c client.Reader, value telemetryv1alpha1.ValueType) ([]byte, error) {
	if value.Value != "" {
		return []byte(value.Value), nil
	}

	if value.ValueFrom == nil || !value.ValueFrom.IsSecretKeyRef() {
		return nil, ErrValueResolveFailed
	}

	valueFromSecret, err := secretref.GetValue(ctx, c, *value.ValueFrom.SecretKeyRef)
	if err != nil {
		return nil, ErrValueResolveFailed
	}

	return valueFromSecret, nil
}
