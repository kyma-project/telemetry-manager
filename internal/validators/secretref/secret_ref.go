package secretref

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
)

type Getter interface {
	GetSecretRefs() []telemetryv1alpha1.SecretKeyRef
}

type Validator struct {
	Client client.Reader
}

var (
	ErrSecretKeyNotFound      = errors.New("one or more keys in a referenced Secret are missing")
	ErrSecretRefNotFound      = errors.New("one or more referenced Secrets are missing")
	ErrSecretRefMissingFields = errors.New("secret reference is missing field/s")
)

func (v *Validator) Validate(ctx context.Context, getter Getter) error {
	refs := getter.GetSecretRefs()
	for _, ref := range refs {
		if _, err := GetValue(ctx, v.Client, ref); err != nil {
			return err
		}
	}

	return nil
}

func GetValue(ctx context.Context, client client.Reader, ref telemetryv1alpha1.SecretKeyRef) ([]byte, error) {
	if err := checkForMissingFields(ref); err != nil {
		return nil, err
	}

	var secret corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("%w: Secret '%s' of Namespace '%s'", ErrSecretRefNotFound, ref.Name, ref.Namespace)
		}

		return nil, &errortypes.APIRequestFailedError{
			Err: fmt.Errorf("failed to get Secret '%s' of Namespace '%s': %w", ref.Name, ref.Namespace, err),
		}
	}

	if secretValue, found := secret.Data[ref.Key]; found {
		return secretValue, nil
	}

	return nil, fmt.Errorf("%w: Key '%s' in Secret '%s' of Namespace '%s'", ErrSecretKeyNotFound, ref.Key, ref.Name, ref.Namespace)
}

func checkForMissingFields(ref telemetryv1alpha1.SecretKeyRef) error {
	var missingAttributes []string

	if ref.Name == "" {
		missingAttributes = append(missingAttributes, "Name")
	}

	if ref.Namespace == "" {
		missingAttributes = append(missingAttributes, "Namespace")
	}

	if ref.Key == "" {
		missingAttributes = append(missingAttributes, "Key")
	}

	if len(missingAttributes) > 0 {
		return fmt.Errorf("%w: %s", ErrSecretRefMissingFields, strings.Join(missingAttributes, ", "))
	}

	return nil
}
