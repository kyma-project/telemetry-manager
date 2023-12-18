package secretref

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type Getter interface {
	GetSecretRefs() []telemetryv1alpha1.SecretKeyRef
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
		if ref.Name == secretName && ref.Namespace == secretNamespace {
			return true
		}
	}

	return false
}

func GetValue(ctx context.Context, client client.Reader, ref telemetryv1alpha1.SecretKeyRef) ([]byte, error) {
	var secret corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, &secret); err != nil {
		return nil, err
	}

	if secretValue, found := secret.Data[ref.Key]; found {
		return secretValue, nil
	}
	return nil, fmt.Errorf("referenced key not found in secret")
}

func checkIfSecretHasKey(ctx context.Context, client client.Reader, ref telemetryv1alpha1.SecretKeyRef) bool {
	var secret corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, &secret); err != nil {
		logf.FromContext(ctx).V(1).Info(fmt.Sprintf("Unable to get secret '%s' from namespace '%s'", ref.Name, ref.Namespace))
		return false
	}
	if _, ok := secret.Data[ref.Key]; !ok {
		logf.FromContext(ctx).V(1).Info(fmt.Sprintf("Unable to find key '%s' in secret '%s'", ref.Key, ref.Name))
		return false
	}

	return true
}
