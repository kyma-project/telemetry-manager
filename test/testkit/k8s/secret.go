package k8s

import (
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit"
)

type Secret struct {
	name       string
	namespace  string
	secretType corev1.SecretType
	stringData map[string]string
	persistent bool
}

func NewOpaqueSecret(name, namespace string, opts ...testkit.OptFunc) *Secret {
	options := processSecretOptions(opts...)

	return &Secret{
		name:       name + uuid.New().String(),
		namespace:  namespace,
		secretType: corev1.SecretTypeOpaque,
		stringData: options.stringData,
	}
}

func (s *Secret) K8sObject() *corev1.Secret {
	var labels Labels
	if s.persistent {
		labels = PersistentLabel
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
			Labels:    labels,
		},
		Type:       s.secretType,
		StringData: s.stringData,
	}
}

func (s *Secret) SecretKeyRef(key string) *telemetryv1alpha1.SecretKeyRef {
	return &telemetryv1alpha1.SecretKeyRef{
		Name:      s.name,
		Namespace: s.namespace,
		Key:       key,
	}
}

func (s *Secret) Persistent(p bool) *Secret {
	s.persistent = p

	return s
}
