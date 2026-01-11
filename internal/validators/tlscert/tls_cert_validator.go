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

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"

	_ "crypto"
)

var (
	ErrCertDecodeFailed = errors.New("failed to decode PEM block containing certificate")
	ErrCertParseFailed  = errors.New("failed to parse certificate")

	ErrKeyDecodeFailed = errors.New("failed to decode PEM block containing private key")

	ErrCADecodeFailed = errors.New("failed to decode PEM block containing CA certificate")
	ErrCAParseFailed  = errors.New("failed to parse CA certificate")

	ErrMissingCertKeyPair = errors.New("a certificate and private key must either both be provided or both be missing")

	ErrValueResolveFailed = errors.New("failed to resolve value")

	ErrInvalidCertificateKeyPair = errors.New("certificate and private key do not match")

	ErrCertIsNotCA = errors.New("not a CA certificate")
)

type TLSValidationParams struct {
	Cert *telemetryv1beta1.ValueType
	Key  *telemetryv1beta1.ValueType
	CA   *telemetryv1beta1.ValueType
}

const twoWeeks = time.Hour * 24 * 7 * 2

const (
	CertExpiredErrorMessage  = "TLS certificate expired on %s"
	CertAboutToExpireMessage = "TLS certificate is about to expire, configured certificate is valid until %s"
	CaExpiredErrorMessage    = "TLS CA certificate expired on %s"
	CaAboutToExpireMessage   = "TLS CA certificate is about to expire, configured certificate is valid until %s"
)

type CertExpiredError struct {
	Expiry time.Time
	IsCa   bool
}

func (cee *CertExpiredError) Error() string {
	if cee.IsCa {
		return fmt.Sprintf(CaExpiredErrorMessage, cee.Expiry.Format(time.DateOnly))
	}

	return fmt.Sprintf(CertExpiredErrorMessage, cee.Expiry.Format(time.DateOnly))
}

func IsCertExpiredError(err error) bool {
	var errCertExpired *CertExpiredError
	return errors.As(err, &errCertExpired)
}

type CertAboutToExpireError struct {
	Expiry time.Time
	IsCa   bool
}

func (cate *CertAboutToExpireError) Error() string {
	if cate.IsCa {
		return fmt.Sprintf(CaAboutToExpireMessage, cate.Expiry.Format(time.DateOnly))
	}

	return fmt.Sprintf(CertAboutToExpireMessage, cate.Expiry.Format(time.DateOnly))
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

func (v *Validator) Validate(ctx context.Context, tls TLSValidationParams) error {
	// 1. Values Resolution
	if (tls.Cert == nil) != (tls.Key == nil) {
		return ErrMissingCertKeyPair
	}

	certPEM, keyPEM, caPEM, err := resolveValues(ctx, v.client, tls)
	if err != nil {
		return err
	}

	// 2. Sanitization and Parsing
	sanitizedCert := sanitizeValue(certPEM)
	sanitizedKey := sanitizeValue(keyPEM)
	sanitizedCA := sanitizeValue(caPEM)

	parsedCert, err := parseCertificate(sanitizedCert)
	if err != nil {
		return err
	}

	if err := parsePrivateKey(sanitizedKey); err != nil {
		return err
	}

	parsedCA, err := parseCA(sanitizedCA)
	if err != nil {
		return err
	}

	// 3. Validation
	if tls.Cert != nil && tls.Key != nil {
		if err := validateCertKeyPair(sanitizedCert, sanitizedKey); err != nil {
			return err
		}

		if err := validateCertificate(parsedCert, v.now()); err != nil {
			return err
		}
	}

	if tls.CA == nil {
		return nil
	}

	for _, ca := range parsedCA {
		if err := validateCA(ca, v.now()); err != nil {
			return err
		}
	}

	return nil
}

func sanitizeValue(valuePEM []byte) []byte {
	return bytes.ReplaceAll(valuePEM, []byte("\\n"), []byte("\n"))
}

func parseCertificate(certPEM []byte) (*x509.Certificate, error) {
	if certPEM == nil {
		return &x509.Certificate{}, nil
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, ErrCertDecodeFailed
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, ErrCertParseFailed
	}

	return cert, nil
}

func parsePrivateKey(keyPEM []byte) error {
	if keyPEM == nil {
		return nil
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return ErrKeyDecodeFailed
	}

	return nil
}

func parseCA(caPEM []byte) ([]*x509.Certificate, error) {
	if caPEM == nil {
		return []*x509.Certificate{}, nil
	}

	block, _ := pem.Decode(caPEM)
	if block == nil {
		return nil, ErrCADecodeFailed
	}

	certs, err := x509.ParseCertificates(block.Bytes)
	if err != nil {
		return nil, ErrCAParseFailed
	}

	return certs, nil
}

func validateCertKeyPair(sanitizedCert, sanitizedKey []byte) error {
	_, err := tls.X509KeyPair(sanitizedCert, sanitizedKey)
	if err != nil {
		return ErrInvalidCertificateKeyPair
	}

	return nil
}

func validateCertificate(cert *x509.Certificate, now time.Time) error {
	certExpiry := cert.NotAfter
	if now.After(certExpiry) {
		return &CertExpiredError{Expiry: certExpiry, IsCa: false}
	}

	if certExpiry.Sub(now) <= twoWeeks {
		return &CertAboutToExpireError{Expiry: certExpiry, IsCa: false}
	}

	return nil
}

func validateCA(ca *x509.Certificate, now time.Time) error {
	if !ca.IsCA {
		return ErrCertIsNotCA
	}

	caExpiry := ca.NotAfter
	if now.After(caExpiry) {
		return &CertExpiredError{Expiry: caExpiry, IsCa: true}
	}

	if caExpiry.Sub(now) <= twoWeeks {
		return &CertAboutToExpireError{Expiry: caExpiry, IsCa: true}
	}

	return nil
}

func resolveValues(ctx context.Context, c client.Reader, tls TLSValidationParams) ([]byte, []byte, []byte, error) {
	var certPEM, keyPEM, caPEM []byte

	var err error

	if tls.Cert != nil {
		certPEM, err = resolveValue(ctx, c, *tls.Cert)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	if tls.Key != nil {
		keyPEM, err = resolveValue(ctx, c, *tls.Key)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	if tls.CA != nil {
		caPEM, err = resolveValue(ctx, c, *tls.CA)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	return certPEM, keyPEM, caPEM, nil
}

func resolveValue(ctx context.Context, c client.Reader, value telemetryv1beta1.ValueType) ([]byte, error) {
	if value.Value != "" {
		return []byte(value.Value), nil
	}

	if value.ValueFrom == nil || value.ValueFrom.SecretKeyRef == nil {
		return nil, ErrValueResolveFailed
	}

	valueFromSecret, err := secretref.GetValue(ctx, c, *value.ValueFrom.SecretKeyRef)
	if err != nil {
		return nil, ErrValueResolveFailed
	}

	return valueFromSecret, nil
}
