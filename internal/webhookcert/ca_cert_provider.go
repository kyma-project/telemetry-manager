package webhookcert

import (
	"context"
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

type caCertGenerator interface {
	generateCert() (certPEM, keyPEM []byte, err error)
}

type caCertProvider struct {
	client    client.Client
	checker   certExpiryChecker
	generator caCertGenerator
}

func newCACertProvider(client client.Client) *caCertProvider {
	clock := realClock{}
	const duration30d = 30 * 24 * time.Hour
	return &caCertProvider{
		client:  client,
		checker: &certExpiryCheckerImpl{clock: realClock{}, timeLeft: duration30d},
		generator: &caCertGeneratorImpl{
			clock: clock,
		},
	}
}

func (p *caCertProvider) provideCert(ctx context.Context, caSecretName types.NamespacedName) ([]byte, []byte, error) {
	var caSecret corev1.Secret
	notFound := false
	if err := p.client.Get(ctx, caSecretName, &caSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, nil, fmt.Errorf("failed to find ca cert caSecretName: %w", err)
		}
		notFound = true
	}

	if notFound || !p.checkCASecret(ctx, &caSecret) {
		caCertPEM, caKeyPEM, err := p.generator.generateCert()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generateCert ca cert: %w", err)
		}

		logf.FromContext(ctx).V(1).Info("Generated new CA cert/key",
			"secretName", caSecretName.Name,
			"secretNamespace", caSecretName.Namespace)

		newSecret := makeCASecret(caCertPEM, caKeyPEM, caSecretName)
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

func (p *caCertProvider) checkCASecret(ctx context.Context, caSecret *corev1.Secret) bool {
	caCertPEM, err := p.fetchCACert(caSecret)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Invalid ca secret. Creating a new one",
			"secretName", caSecret.Name,
			"secretNamespace", caSecret.Namespace)
		return false
	}

	valid, err := p.checker.checkExpiry(ctx, caCertPEM)
	return err == nil && valid
}

func (p *caCertProvider) fetchCACert(caSecret *corev1.Secret) ([]byte, error) {
	var caCertPEM []byte
	if val, found := caSecret.Data[caCertFile]; found {
		caCertPEM = val
	} else {
		return nil, fmt.Errorf("key not found: %v", caCertFile)
	}

	if _, found := caSecret.Data[caKeyFile]; !found {
		return nil, fmt.Errorf("key not found: %v", caKeyFile)
	}

	return caCertPEM, nil
}

func makeCASecret(certificate []byte, key []byte, name types.NamespacedName) corev1.Secret {
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
