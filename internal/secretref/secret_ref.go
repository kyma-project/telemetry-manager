package secretref

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/field"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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

func FetchReferencedSecretData(ctx context.Context, client client.Reader, getter Getter) (map[string][]byte, error) {
	secretData := map[string][]byte{}

	refs := getter.GetSecretRefs()
	for _, ref := range refs {
		secretValue, err := fetchSecretValue(ctx, client, ref.SecretKeyRef)
		if err != nil {
			return nil, err
		}
		secretData[ref.TargetSecretKey] = secretValue
	}
	return secretData, nil
}

func fetchSecretValue(ctx context.Context, client client.Reader, from field.SecretKeyRef) ([]byte, error) {
	var secret corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: from.Name, Namespace: from.Namespace}, &secret); err != nil {
		return nil, err
	}

	if secretValue, found := secret.Data[from.Key]; found {
		return secretValue, nil
	}
	return nil, fmt.Errorf("referenced key not found in Secret")
}

func checkIfSecretHasKey(ctx context.Context, client client.Reader, from field.SecretKeyRef) bool {
	log := logf.FromContext(ctx)

	var secret corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: from.Name, Namespace: from.Namespace}, &secret); err != nil {
		log.V(1).Info(fmt.Sprintf("Unable to get secret '%s' from namespace '%s'", from.Name, from.Namespace))
		return false
	}
	if _, ok := secret.Data[from.Key]; !ok {
		log.V(1).Info(fmt.Sprintf("Unable to find key '%s' in secret '%s'", from.Key, from.Name))
		return false
	}

	return true
}
