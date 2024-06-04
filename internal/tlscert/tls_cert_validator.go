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

	ErrValueResolveFailed = errors.New("failed to resolve value")

	ErrInvalidCertificateKeyPair = errors.New("certificate and private key do not match")

	ErrCertIsNotCA   = errors.New("not a CA certificate")
	ErrCASubject     = errors.New("CA subject does not match certificate issuer")
)

const twoWeeks = time.Hour * 24 * 7 * 2

type CertExpiredError struct {
	Expiry time.Time
}

type TLSConfig struct {
	Cert *telemetryv1alpha1.ValueType
	Key  *telemetryv1alpha1.ValueType
	CA   *telemetryv1alpha1.ValueType
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

func (v *Validator) ValidateCertificate(ctx context.Context, config TLSConfig) error {
	certPEM, err := resolveValue(ctx, v.client, *config.Cert)
	if err != nil {
		return err
	}

	keyPEM, err := resolveValue(ctx, v.client, *config.Key)
	if err != nil {
		return err
	}

	caPEM, err := resolveValue(ctx, v.client, *config.CA)
	if err != nil {
		return err
	}

	// Make the best effort replacement of linebreaks in cert/key if present.
	sanitizedCert := bytes.ReplaceAll(certPEM, []byte("\\n"), []byte("\n"))
	sanitizedKey := bytes.ReplaceAll(keyPEM, []byte("\\n"), []byte("\n"))
	sanitizedCA := bytes.ReplaceAll(caPEM, []byte("\\n"), []byte("\n"))

	// Parse the certificate
	parsedCert, err := parseCertificate(sanitizedCert, ErrCertDecodeFailed, ErrCertParseFailed)
	if err != nil {
		return err
	}

	// Parse the private key
	if err = parsePrivateKey(sanitizedKey); err != nil {
		return err
	}

	// Parse the CA
	parsedCA, err := parseCertificate(sanitizedCA, ErrCADecodeFailed, ErrCAParseFailed)
	if err != nil {
		return err
	}

	// Validate the certificate-key pair
	_, err = tls.X509KeyPair(sanitizedCert, sanitizedKey)
	if err != nil {
		return ErrInvalidCertificateKeyPair
	}

	// Validate the CA
	if err = validateCA(parsedCert, parsedCA); err != nil {
		return err
	}

	// Validate certificate expiry
	certExpiry := parsedCert.NotAfter
	if v.now().After(certExpiry) {
		return &CertExpiredError{Expiry: certExpiry}
	}
	if certExpiry.Sub(v.now()) <= twoWeeks {
		return &CertAboutToExpireError{Expiry: certExpiry}
	}

	return nil
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

func validateCA(cert *x509.Certificate, ca *x509.Certificate) error {
	if !ca.IsCA {
		return ErrCertIsNotCA
	}

	if cert.Issuer.String() != ca.Subject.String() {
		return ErrCASubject
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
