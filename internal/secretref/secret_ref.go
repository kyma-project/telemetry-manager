package secretref

import (
	"context"
	"github.com/kyma-project/telemetry-manager/internal/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Getter interface {
	GetSecretRefs() []field.Descriptor
}

func ReferencesNonExistentSecret(ctx context.Context, client client.Reader, getter Getter) bool {
	refs := getter.GetSecretRefs()
	for _, ref := range refs {
		hasKey := checkIfSecretHasKey(ctx, client, ref.SecretKeyRef)
		if !hasKey {
			return true
		}
	}

	return false
}

func ReferencesSecret(secretName, secretNamespace string, getter Getter) bool {
	refs := getter.GetSecretRefs()

	for _, ref := range refs {
		if ref.SecretKeyRef.Name == secretName && ref.SecretKeyRef.Namespace == secretNamespace {
			return true
		}
	}

	return false
}
