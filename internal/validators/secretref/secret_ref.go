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

type Validator struct {
	Client client.Reader
}

var (
	ErrSecretKeyNotFound      = errors.New("one or more keys in a referenced Secret are missing")
	ErrSecretRefNotFound      = errors.New("one or more referenced Secrets are missing")
	ErrSecretRefMissingFields = errors.New("secret reference is missing field/s")
)

// ValidateTracePipeline validates the secret references in a TracePipeline, ensuring that the references are valid,
// and the referenced Secrets exist and contain the required keys. It returns an error otherwise.
func (v *Validator) ValidateTracePipeline(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) error {
	return v.validate(ctx, getSecretRefsTracePipeline(pipeline))
}

// ValidateMetricPipeline validates the secret references in a MetricPipeline, ensuring that the references are valid,
// and the referenced Secrets exist and contain the required keys. It returns an error otherwise.
func (v *Validator) ValidateMetricPipeline(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) error {
	return v.validate(ctx, getSecretRefsMetricPipeline(pipeline))
}

// ValidateLogPipeline validates the secret references in a LogPipeline, ensuring that the references are valid,
// and the referenced Secrets exist and contain the required keys. It returns an error otherwise.
func (v *Validator) ValidateLogPipeline(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	return v.validate(ctx, getSecretRefsLogPipeline(pipeline))
}

func (v *Validator) validate(ctx context.Context, refs []telemetryv1alpha1.SecretKeyRef) error {
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

func getSecretRefsTracePipeline(tp *telemetryv1alpha1.TracePipeline) []telemetryv1alpha1.SecretKeyRef {
	return getSecretRefsInOTLPOutput(tp.Spec.Output.OTLP)
}

func getSecretRefsMetricPipeline(mp *telemetryv1alpha1.MetricPipeline) []telemetryv1alpha1.SecretKeyRef {
	return getSecretRefsInOTLPOutput(mp.Spec.Output.OTLP)
}

func getSecretRefsLogPipeline(lp *telemetryv1alpha1.LogPipeline) []telemetryv1alpha1.SecretKeyRef {
	var refs []telemetryv1alpha1.SecretKeyRef

	for _, v := range lp.Spec.Variables {
		if v.ValueFrom.SecretKeyRef != nil {
			refs = append(refs, *v.ValueFrom.SecretKeyRef)
		}
	}

	refs = append(refs, getSecretRefsInHTTPOutput(lp.Spec.Output.HTTP)...)

	return refs
}

func getSecretRefsInHTTPOutput(httpOutput *telemetryv1alpha1.LogPipelineHTTPOutput) []telemetryv1alpha1.SecretKeyRef {
	var refs []telemetryv1alpha1.SecretKeyRef

	if httpOutput != nil {
		refs = appendIfSecretRef(refs, httpOutput.Host)
		refs = appendIfSecretRef(refs, httpOutput.User)
		refs = appendIfSecretRef(refs, httpOutput.Password)

		tlsConfig := httpOutput.TLS
		if tlsConfig.CA != nil {
			refs = appendIfSecretRef(refs, *tlsConfig.CA)
		}

		if tlsConfig.Cert != nil {
			refs = appendIfSecretRef(refs, *tlsConfig.Cert)
		}

		if tlsConfig.Key != nil {
			refs = appendIfSecretRef(refs, *tlsConfig.Key)
		}
	}

	return refs
}

func getSecretRefsInOTLPOutput(otlpOut *telemetryv1alpha1.OTLPOutput) []telemetryv1alpha1.SecretKeyRef {
	var refs []telemetryv1alpha1.SecretKeyRef

	refs = appendIfSecretRef(refs, otlpOut.Endpoint)

	if otlpOut.Authentication != nil && otlpOut.Authentication.Basic != nil {
		refs = appendIfSecretRef(refs, otlpOut.Authentication.Basic.User)
		refs = appendIfSecretRef(refs, otlpOut.Authentication.Basic.Password)
	}

	for _, header := range otlpOut.Headers {
		refs = appendIfSecretRef(refs, header.ValueType)
	}

	if otlpOut.TLS != nil && !otlpOut.TLS.Insecure {
		if otlpOut.TLS.CA != nil {
			refs = appendIfSecretRef(refs, *otlpOut.TLS.CA)
		}

		if otlpOut.TLS.Cert != nil {
			refs = appendIfSecretRef(refs, *otlpOut.TLS.Cert)
		}

		if otlpOut.TLS.Key != nil {
			refs = appendIfSecretRef(refs, *otlpOut.TLS.Key)
		}
	}

	return refs
}

func appendIfSecretRef(secretKeyRefs []telemetryv1alpha1.SecretKeyRef, valueType telemetryv1alpha1.ValueType) []telemetryv1alpha1.SecretKeyRef {
	if valueType.Value == "" && valueType.ValueFrom != nil && valueType.ValueFrom.SecretKeyRef != nil {
		secretKeyRefs = append(secretKeyRefs, *valueType.ValueFrom.SecretKeyRef)
	}

	return secretKeyRefs
}
