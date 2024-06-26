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
	ErrMissingAll     = errors.New("no TLS configuration provided, missing certificate, private key, and CA")
	ErrMissingCertKey = errors.New("missing certificate or/and private key, and CA was not provided")

	ErrCertDecodeFailed = errors.New("failed to decode PEM block containing certificate")
	ErrCertParseFailed  = errors.New("failed to parse certificate")

	ErrKeyDecodeFailed = errors.New("failed to decode PEM block containing private key")
	ErrKeyParseFailed  = errors.New("failed to parse private key")

	ErrCADecodeFailed = errors.New("failed to decode PEM block containing CA certificate")
	ErrCAParseFailed  = errors.New("failed to parse CA certificate")

	ErrValueResolveFailed = errors.New("failed to resolve value")

	ErrInvalidCertificateKeyPair = errors.New("certificate and private key do not match")

	ErrCertIsNotCA = errors.New("not a CA certificate")
)

type TLSBundle struct {
	Cert *telemetryv1alpha1.ValueType
	Key  *telemetryv1alpha1.ValueType
	CA   *telemetryv1alpha1.ValueType
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
	// Check for missing configuration
	missingCert, missingKey, missingCA, err := checkForMissingConfig(config)
	if err != nil {
		return err
	}

	// Resolve cert value (if not missing)
	var certPEM []byte
	if !missingCert {
		certPEM, err = resolveValue(ctx, v.client, *config.Cert)
	}
	if err != nil {
		return err
	}

	// Resolve key value (if not missing)
	var keyPEM []byte
	if !missingKey {
		keyPEM, err = resolveValue(ctx, v.client, *config.Key)
	}
	if err != nil {
		return err
	}

	// Resolve CA value (if not missing)
	var caPEM []byte
	if !missingCA {
		caPEM, err = resolveValue(ctx, v.client, *config.CA)
	}
	if err != nil {
		return err
	}

	// Make the best effort to replace linebreaks in cert/key/ca if present.
	sanitizedCert := bytes.ReplaceAll(certPEM, []byte("\\n"), []byte("\n"))
	sanitizedKey := bytes.ReplaceAll(keyPEM, []byte("\\n"), []byte("\n"))
	sanitizedCA := bytes.ReplaceAll(caPEM, []byte("\\n"), []byte("\n"))

	// Parse the certificate (if not missing)
	var parsedCert *x509.Certificate
	if !missingCert {
		parsedCert, err = parseCertificate(sanitizedCert, ErrCertDecodeFailed, ErrCertParseFailed)
	}
	if err != nil {
		return err
	}

	// Parse the private key (if not missing)
	if !missingKey {
		err = parsePrivateKey(sanitizedKey)
	}
	if err != nil {
		return err
	}

	// Parse the CA(s) (if not missing)
	var parsedCAs []*x509.Certificate
	if !missingCA {
		parsedCAs, err = parseCertificates(sanitizedCA, ErrCADecodeFailed, ErrCAParseFailed)
	}
	if err != nil {
		return err
	}

	// Validate certificate (if not missing)
	if !missingCert && !missingKey {
		err = validateCertificate(parsedCert, sanitizedCert, sanitizedKey, v.now())
	}
	if err != nil {
		return err
	}

	// Validate CA(s) (if not missing)
	if missingCA {
		return nil
	}
	for _, ca := range parsedCAs {
		if err := validateCA(ca, v.now()); err != nil {
			return err
		}
	}

	// Validation successful
	return nil
}

func checkForMissingConfig(config TLSBundle) bool, bool, bool, error {
	missingCert := config.Cert == nil
	missingKey := config.Key == nil
	missingCA := config.CA == nil

	if missingCert && missingKey && missingCA {
		return missingCert, missingKey, missingCA, ErrMissingAll
	}
	if (missingCert && !missingKey) || (!missingCert && missingKey) {
		return missingCert, missingKey, missingCA, ErrMissingCertKey
	}

	return missingCert, missingKey, missingCA, nil
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
