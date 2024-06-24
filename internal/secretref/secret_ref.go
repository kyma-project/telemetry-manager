package secretref

import (
	"context"
	"errors"
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

var (
	ErrSecretKeyNotFound = errors.New("One or more keys in a referenced Secret are missing") //nolint:stylecheck //Message will be used in condition message and must be capitalized
	ErrSecretRefNotFound = errors.New("One or more referenced Secrets are missing")          //nolint:stylecheck //Message will be used in condition message and must be capitalized
)

func VerifySecretReference(ctx context.Context, client client.Reader, getter Getter) error {
	refs := getter.GetSecretRefs()
	for _, ref := range refs {
		if _, err := GetValue(ctx, client, ref); err != nil {
			return err
		}
	}

	return nil
}

func GetValue(ctx context.Context, client client.Reader, ref telemetryv1alpha1.SecretKeyRef) ([]byte, error) {
	var secret corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, &secret); err != nil {
		logf.FromContext(ctx).V(1).Info(fmt.Sprintf("Unable to get Secret '%s' from Namespace '%s'", ref.Name, ref.Namespace))
		return nil, fmt.Errorf("%w: Secret '%s' of Namespace '%s'", ErrSecretRefNotFound, ref.Name, ref.Namespace)
	}

	if secretValue, found := secret.Data[ref.Key]; found {
		return secretValue, nil
	}
	logf.FromContext(ctx).V(1).Info(fmt.Sprintf("Unable to find key '%s' in Secret '%s' from Namespace '%s'", ref.Key, ref.Name, ref.Namespace))
	return nil, fmt.Errorf("%w: Key '%s' in Secret '%s' of Namespace '%s'", ErrSecretKeyNotFound, ref.Key, ref.Name, ref.Namespace)
}
