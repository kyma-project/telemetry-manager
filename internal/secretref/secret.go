package secretref

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/field"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func fetchSecretValue(ctx context.Context, client client.Reader, value telemetryv1alpha1.ValueType) ([]byte, error) {
	if value.Value != "" {
		return []byte(value.Value), nil
	}
	if value.ValueFrom.IsSecretKeyRef() {
		lookupKey := types.NamespacedName{
			Name:      value.ValueFrom.SecretKeyRef.Name,
			Namespace: value.ValueFrom.SecretKeyRef.Namespace,
		}

		var secret corev1.Secret
		if err := client.Get(ctx, lookupKey, &secret); err != nil {
			return nil, err
		}

		if secretValue, found := secret.Data[value.ValueFrom.SecretKeyRef.Key]; found {
			return secretValue, nil
		}
		return nil, fmt.Errorf("referenced key not found in Secret")
	}

	return nil, fmt.Errorf("either value or secretReference have to be defined")
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
