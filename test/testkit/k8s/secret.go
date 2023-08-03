//go:build e2e

package k8s

import (
	"github.com/google/uuid"
	k8score "k8s.io/api/core/v1"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetry "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit"
)

type Secret struct {
	name       string
	namespace  string
	secretType k8score.SecretType
	stringData map[string]string
	persistent bool
}

func NewOpaqueSecret(name, namespace string, opts ...testkit.OptFunc) *Secret {
	options := processSecretOptions(opts...)

	return &Secret{
		name:       name + uuid.New().String(),
		namespace:  namespace,
		secretType: k8score.SecretTypeOpaque,
		stringData: options.stringData,
	}
}

func (s *Secret) K8sObject() *k8score.Secret {
	var labels Labels
	if s.persistent {
		labels = PersistentLabel
	}

	return &k8score.Secret{
		ObjectMeta: k8smeta.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
			Labels:    labels,
		},
		Type:       s.secretType,
		StringData: s.stringData,
	}
}

func (s *Secret) SecretKeyRef(key string) *telemetry.SecretKeyRef {
	return &telemetry.SecretKeyRef{
		Name:      s.name,
		Namespace: s.namespace,
		Key:       key,
	}
}

func (s *Secret) Persistent(p bool) *Secret {
	s.persistent = p

	return s
}
