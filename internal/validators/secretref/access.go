package secretref

import (
	"context"
	"fmt"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// AccessValidator checks whether the requesting user has get permission
// on each SecretKeyRef before the pipeline is admitted.
// This prevents privilege escalation through the operator's cluster-wide secret read access.
type AccessValidator struct {
	Client client.Client
}

// ValidateSecretAccess performs a SubjectAccessReview for every unique secret (by name/namespace)
// referenced in refs, using the identity from the admission request context. Refs with a missing
// name or namespace are skipped — field-level validation is handled by the existing Validator.
// Returns an error if the user is not permitted to get any of the referenced secrets.
func (v *AccessValidator) ValidateSecretAccess(ctx context.Context, userInfo authenticationv1.UserInfo, refs []telemetryv1beta1.SecretKeyRef) error {
	complete := make([]telemetryv1beta1.SecretKeyRef, 0, len(refs))
	for _, ref := range refs {
		if ref.Name != "" && ref.Namespace != "" {
			complete = append(complete, ref)
		}
	}

	for _, nn := range RefsToSecretNames(complete) {
		if err := v.canGetSecret(ctx, userInfo, nn); err != nil {
			return err
		}
	}

	return nil
}

func (v *AccessValidator) canGetSecret(ctx context.Context, userInfo authenticationv1.UserInfo, nn types.NamespacedName) error {
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
				Namespace: nn.Namespace,
				Name:      nn.Name,
			},
		},
	}

	if err := v.Client.Create(ctx, sar); err != nil {
		return fmt.Errorf("failed to check secret access for %s/%s: %w", nn.Namespace, nn.Name, err)
	}

	if !sar.Status.Allowed {
		reason := sar.Status.Reason
		if reason == "" {
			reason = "not allowed"
		}

		return fmt.Errorf("user %q is not allowed to get secret %q in namespace %q: %s", userInfo.Username, nn.Name, nn.Namespace, reason)
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
