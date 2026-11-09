package secretref

import (
	"context"
	"fmt"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// AccessValidator checks whether the requesting user has get permission
// on each SecretKeyRef before the pipeline is admitted.
// This prevents privilege escalation through the operator's cluster-wide secret read access.
type AccessValidator struct {
	Client client.Client
}

// ValidateSecretAccess performs a SubjectAccessReview for every SecretKeyRef in refs
// using the identity from the admission request context. Returns an error if the user
// is not permitted to get any of the referenced secrets.
func (v *AccessValidator) ValidateSecretAccess(ctx context.Context, userInfo authenticationv1.UserInfo, refs []telemetryv1beta1.SecretKeyRef) error {
	for _, ref := range refs {
		if err := checkForMissingFields(ref); err != nil {
			// Field-level validation is handled by the existing Validator; skip refs
			// with missing fields so that both validators can report independently.
			continue
		}

		if err := v.canGetSecret(ctx, userInfo, ref); err != nil {
			return err
		}
	}

	return nil
}

func (v *AccessValidator) canGetSecret(ctx context.Context, userInfo authenticationv1.UserInfo, ref telemetryv1beta1.SecretKeyRef) error {
	extra := convertExtra(userInfo.Extra)

	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   userInfo.Username,
			Groups: userInfo.Groups,
			UID:    userInfo.UID,
			Extra:  extra,
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:      "get",
				Resource:  "secrets",
				Namespace: ref.Namespace,
				Name:      ref.Name,
			},
		},
	}

	if err := v.Client.Create(ctx, sar); err != nil {
		return fmt.Errorf("failed to check secret access for %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	if !sar.Status.Allowed {
		reason := sar.Status.Reason
		if reason == "" {
			reason = "not allowed"
		}

		return fmt.Errorf("user %q is not allowed to get secret %q in namespace %q: %s", userInfo.Username, ref.Name, ref.Namespace, reason)
	}

	return nil
}

func convertExtra(extra map[string]authenticationv1.ExtraValue) map[string]authorizationv1.ExtraValue {
	if len(extra) == 0 {
		return nil
	}

	out := make(map[string]authorizationv1.ExtraValue, len(extra))

	for k, v := range extra {
		out[k] = authorizationv1.ExtraValue(v)
	}

	return out
}
