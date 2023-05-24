package webhookcert

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
)

const (
	caCertFile = "ca.crt"
	caKeyFile  = "ca.key"
)

type caCertKeyProvider struct {
	client client.Client
	clock  clock
}

func newCACertKeyProvider(client client.Client) *caCertKeyProvider {
	return &caCertKeyProvider{
		client: client,
		clock:  realClock{},
	}
}

func (p *caCertKeyProvider) provideCACertKey(ctx context.Context, caSecretName types.NamespacedName) ([]byte, []byte, error) {
	var caSecret corev1.Secret
	notFound := false
	if err := p.client.Get(ctx, caSecretName, &caSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, nil, fmt.Errorf("failed to find ca cert caSecretName: %w", err)
		}
		notFound = true
	}

	if notFound || !p.checkCASecret(ctx, &caSecret) {
		caCertPEM, caKeyPEM, err := generateCACertKey()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate ca cert: %w", err)
		}

		logf.FromContext(ctx).Info("Generated new CA cert/key",
			"secretName", caSecretName.Name,
			"secretNamespace", caSecretName.Namespace)

		newSecret := makeCaSecret(caCertPEM, caKeyPEM, caSecretName)
		if err = kubernetes.CreateOrUpdateSecret(ctx, p.client, &newSecret); err != nil {
			return nil, nil, fmt.Errorf("failed to create ca cert caSecretName: %w", err)
		}
		return caCertPEM, caKeyPEM, nil
	}

	logf.FromContext(ctx).Info("Found existing CA cert/key",
		"secretName", caSecretName.Name,
		"secretNamespace", caSecretName.Namespace)

	return caSecret.Data[caCertFile], caSecret.Data[caKeyFile], nil
}

func (p *caCertKeyProvider) checkCASecret(ctx context.Context, caSecret *corev1.Secret) bool {
	if err := p.checkCASecretInternal(caSecret); err != nil {
		logf.FromContext(ctx).Error(err, "Invalid ca secret. Rotating the cert",
			"secretName", caSecret.Name,
			"secretNamespace", caSecret.Namespace)
		return false
	}

	return true
}

func (p *caCertKeyProvider) checkCASecretInternal(caSecret *corev1.Secret) error {
	var caCertPEM, _ []byte
	if val, found := caSecret.Data[caCertFile]; found {
		caCertPEM = val
	} else {
		return fmt.Errorf("key not found: %v", caCertFile)
	}

	if _, found := caSecret.Data[caKeyFile]; !found {
		return fmt.Errorf("key not found: %v", caKeyFile)
	}

	return p.checkCertExpiry(caCertPEM)
}

func (p *caCertKeyProvider) checkCertExpiry(certPEM []byte) error {
	cert, err := parseCertPEM(certPEM)
	if err != nil {
		return err
	}

	aboutToExpireTime := cert.NotAfter.UTC().Add(-1 * 24 * time.Hour)
	if p.clock.now().Before(aboutToExpireTime) {
		return nil
	}
	return errors.New("cert is about to expire: rotation needed")
}

func makeCaSecret(certificate []byte, key []byte, name types.NamespacedName) corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Data: map[string][]byte{
			caCertFile: certificate,
			caKeyFile:  key,
		},
	}
}
