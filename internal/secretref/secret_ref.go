package secretref

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/internal/field"
)

type Getter interface {
	GetSecretRefs() []field.Descriptor
}

func ReferencesNonExistentSecret(ctx context.Context, client client.Reader, getter Getter) bool {
	refs := getter.GetSecretRefs()
	for _, ref := range refs {
		hasKey := checkIfSecretHasKey(ctx, client, ref)
		if !hasKey {
			return true
		}
	}

	return false
}

func ReferencesSecret(secretName, secretNamespace string, getter Getter) bool {
	refs := getter.GetSecretRefs()
	for _, ref := range refs {
		if ref.SourceSecretName == secretName && ref.SourceSecretNamespace == secretNamespace {
			return true
		}
	}

	return false
}

func FetchReferencedSecretData(ctx context.Context, client client.Reader, getter Getter) (map[string][]byte, error) {
	secretData := map[string][]byte{}

	refs := getter.GetSecretRefs()
	for _, ref := range refs {
		secretValue, err := fetchSecretValue(ctx, client, ref)
		if err != nil {
			return nil, err
		}
		secretData[ref.TargetSecretKey] = secretValue
	}
	return secretData, nil
}

func fetchSecretValue(ctx context.Context, client client.Reader, ref field.Descriptor) ([]byte, error) {
	var secret corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: ref.SourceSecretName, Namespace: ref.SourceSecretNamespace}, &secret); err != nil {
		return nil, err
	}

	if secretValue, found := secret.Data[ref.SourceSecretKey]; found {
		return secretValue, nil
	}
	return nil, fmt.Errorf("referenced key not found in Secret")
}

func checkIfSecretHasKey(ctx context.Context, client client.Reader, ref field.Descriptor) bool {
	log := logf.FromContext(ctx)

	var secret corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: ref.SourceSecretName, Namespace: ref.SourceSecretNamespace}, &secret); err != nil {
		log.V(1).Info(fmt.Sprintf("Unable to get secret '%s' from namespace '%s'", ref.SourceSecretName, ref.SourceSecretNamespace))
		return false
	}
	if _, ok := secret.Data[ref.SourceSecretKey]; !ok {
		log.V(1).Info(fmt.Sprintf("Unable to find key '%s' in secret '%s'", ref.SourceSecretKey, ref.SourceSecretName))
		return false
	}

	return true
}
