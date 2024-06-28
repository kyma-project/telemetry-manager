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
	ErrCertDecodeFailed = errors.New("failed to decode PEM block containing certificate")
	ErrCertParseFailed  = errors.New("failed to parse certificate")

	ErrKeyDecodeFailed = errors.New("failed to decode PEM block containing private key")
	ErrKeyParseFailed  = errors.New("failed to parse private key")

	ErrCADecodeFailed = errors.New("failed to decode PEM block containing CA certificate")
	ErrCAParseFailed  = errors.New("failed to parse CA certificate")

	ErrMissingCertKeyPair = errors.New("a certificate and private key must either both be provided or both be missing")

	ErrValueResolveFailed = errors.New("failed to resolve value")

	ErrInvalidCertificateKeyPair = errors.New("certificate and private key do not match")

	ErrCertIsNotCA = errors.New("not a CA certificate")
)

type TLSBundle struct {
	Cert *telemetryv1alpha1.ValueType
	Key  *telemetryv1alpha1.ValueType
	CA   *telemetryv1alpha1.ValueType
}

func (b *TLSBundle) GetMissing() (bool, bool, bool) {
	return b.Cert == nil, b.Key == nil, b.CA == nil
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

func (v *Validator) Validate(ctx context.Context, config TLSBundle) error {
	// Check for missing TLS configuration values
	err := checkForMissingValues(config)
	if err != nil {
		return err
	}

	// Resolve values
	certPEM, keyPEM, caPEM, err := resolveValues(ctx, v.client, config)
	if err != nil {
		return err
	}

	// Make the best effort to replace linebreaks in cert/key/ca if present.
	sanitizedCert := bytes.ReplaceAll(certPEM, []byte("\\n"), []byte("\n"))
	sanitizedKey := bytes.ReplaceAll(keyPEM, []byte("\\n"), []byte("\n"))
	sanitizedCA := bytes.ReplaceAll(caPEM, []byte("\\n"), []byte("\n"))

	// Parse values
	parsedCert, parsedCAs, err := parseValues(sanitizedCert, sanitizedKey, sanitizedCA)
	if err != nil {
		return err
	}

	// Validate values
	err = validateValues(config, parsedCert, parsedCAs, sanitizedCert, sanitizedKey, v.now())
	if err != nil {
		return err
	}

	return nil
}

func checkForMissingValues(config TLSBundle) error {
	missingCert, missingKey, _ := config.GetMissing()

	if missingCert != missingKey {
		return ErrMissingCertKeyPair
	}

	return nil
}

func parseValues(sanitizedCert, sanitizedKey, sanitizedCA []byte) (*x509.Certificate, []*x509.Certificate, error) {
	var parsedCert *x509.Certificate
	var parsedCAs []*x509.Certificate
	var err error

	// Parse the certificate (if not missing)
	if sanitizedCert != nil {
		parsedCert, err = parseCertificate(sanitizedCert, ErrCertDecodeFailed, ErrCertParseFailed)
	}
	if err != nil {
		return parsedCert, parsedCAs, err
	}

	// Parse the private key (if not missing)
	if sanitizedKey != nil {
		err = parsePrivateKey(sanitizedKey)
	}
	if err != nil {
		return parsedCert, parsedCAs, err
	}

	// Parse the CA(s) (if not missing)
	if sanitizedCA != nil {
		parsedCAs, err = parseCertificates(sanitizedCA, ErrCADecodeFailed, ErrCAParseFailed)
	}
	if err != nil {
		return parsedCert, parsedCAs, err
	}

	return parsedCert, parsedCAs, nil
}

func parseCertificate(certPEM []byte, errDecode error, errParse error) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, errDecode
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return cert, errParse
	}

	return cert, nil
}

func parseCertificates(certPEM []byte, errDecode error, errParse error) ([]*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, errDecode
	}

	certs, err := x509.ParseCertificates(block.Bytes)
	if err != nil {
		return certs, errParse
	}

	return certs, nil
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

func validateValues(config TLSBundle, parsedCert *x509.Certificate, parsedCAs []*x509.Certificate, sanitizedCert, sanitizedKey []byte, now time.Time) error {
	missingCert, missingKey, missingCA := config.GetMissing()
	var err error

	// Validate certificate (if not missing)
	if !missingCert && !missingKey {
		err = validateCertificate(parsedCert, sanitizedCert, sanitizedKey, now)
	}
	if err != nil {
		return err
	}

	// Validate CA(s) (if not missing)
	if missingCA {
		return nil
	}
	for _, ca := range parsedCAs {
		if err := validateCA(ca, now); err != nil {
			return err
		}
	}

	return nil
}

func validateCertificate(cert *x509.Certificate, sanitizedCert, sanitizedKey []byte, now time.Time) error {
	// Validate the certificate-key pair
	_, err := tls.X509KeyPair(sanitizedCert, sanitizedKey)
	if err != nil {
		return ErrInvalidCertificateKeyPair
	}

	// Validate certificate expiry
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
	// Validate CA flag
	if !ca.IsCA {
		return ErrCertIsNotCA
	}

	// Validate CA expiry
	caExpiry := ca.NotAfter
	if now.After(caExpiry) {
		return &CertExpiredError{Expiry: caExpiry, IsCa: true}
	}
	if caExpiry.Sub(now) <= twoWeeks {
		return &CertAboutToExpireError{Expiry: caExpiry, IsCa: true}
	}

	return nil
}

func resolveValues(ctx context.Context, c client.Reader, config TLSBundle) ([]byte, []byte, []byte, error) {
	var certPEM, keyPEM, caPEM []byte
	var err error

	// Resolve cert value (if not missing)
	if config.Cert != nil {
		certPEM, err = resolveValue(ctx, c, *config.Cert)
	}
	if err != nil {
		return certPEM, keyPEM, caPEM, err
	}

	// Resolve key value (if not missing)
	if config.Key != nil {
		keyPEM, err = resolveValue(ctx, c, *config.Key)
	}
	if err != nil {
		return certPEM, keyPEM, caPEM, err
	}

	// Resolve CA value (if not missing)
	if config.CA != nil {
		caPEM, err = resolveValue(ctx, c, *config.CA)
	}
	if err != nil {
		return certPEM, keyPEM, caPEM, err
	}

	return certPEM, keyPEM, caPEM, nil
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
