package secretref

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// sarAllowingClient returns a fake client that, on SubjectAccessReview Create, sets Status.Allowed
// to the given value and optionally a Reason.
func sarAllowingClient(t *testing.T, allowed bool, reason string) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, authorizationv1.AddToScheme(scheme))

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				sar, ok := obj.(*authorizationv1.SubjectAccessReview)
				if !ok {
					return c.Create(ctx, obj, opts...)
				}

				sar.Status = authorizationv1.SubjectAccessReviewStatus{
					Allowed: allowed,
					Reason:  reason,
				}

				return nil
			},
		}).
		Build()
}

func TestAccessValidator_ValidateSecretAccess_Allowed(t *testing.T) {
	v := &AccessValidator{Client: sarAllowingClient(t, true, "")}

	refs := []telemetryv1beta1.SecretKeyRef{
		{Name: "my-secret", Namespace: "default", Key: "password"},
	}
	userInfo := authenticationv1.UserInfo{Username: "alice", Groups: []string{"system:authenticated"}}

	err := v.ValidateSecretAccess(t.Context(), userInfo, refs)
	assert.NoError(t, err)
}

func TestAccessValidator_ValidateSecretAccess_Denied(t *testing.T) {
	v := &AccessValidator{Client: sarAllowingClient(t, false, "user cannot get secrets in kube-system")}

	refs := []telemetryv1beta1.SecretKeyRef{
		{Name: "service-account-token", Namespace: "kube-system", Key: "token"},
	}
	userInfo := authenticationv1.UserInfo{Username: "bob", Groups: []string{"system:authenticated"}}

	err := v.ValidateSecretAccess(t.Context(), userInfo, refs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bob")
	assert.Contains(t, err.Error(), "service-account-token")
	assert.Contains(t, err.Error(), "kube-system")
}

func TestAccessValidator_ValidateSecretAccess_DeniedOnSecondRef(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, authorizationv1.AddToScheme(scheme))

	callCount := 0

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				sar, ok := obj.(*authorizationv1.SubjectAccessReview)
				if !ok {
					return cl.Create(ctx, obj, opts...)
				}

				callCount++
				sar.Status = authorizationv1.SubjectAccessReviewStatus{
					// Allow first ref, deny second
					Allowed: callCount == 1,
				}

				return nil
			},
		}).
		Build()

	v := &AccessValidator{Client: c}
	refs := []telemetryv1beta1.SecretKeyRef{
		{Name: "allowed-secret", Namespace: "default", Key: "key"},
		{Name: "denied-secret", Namespace: "kube-system", Key: "token"},
	}
	userInfo := authenticationv1.UserInfo{Username: "eve"}

	err := v.ValidateSecretAccess(t.Context(), userInfo, refs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied-secret")
	assert.Equal(t, 2, callCount)
}

func TestAccessValidator_ValidateSecretAccess_SkipsMissingFieldRefs(t *testing.T) {
	v := &AccessValidator{Client: sarAllowingClient(t, false, "should not be called")}

	// Ref with missing Name — SAR skipped (no meaningful secret to check).
	refs := []telemetryv1beta1.SecretKeyRef{
		{Name: "", Namespace: "default", Key: "key"},
	}
	userInfo := authenticationv1.UserInfo{Username: "alice"}

	// No SAR was called (would return denied), so no error.
	err := v.ValidateSecretAccess(t.Context(), userInfo, refs)
	assert.NoError(t, err)
}

func TestAccessValidator_ValidateSecretAccess_EmptyKeyIsChecked(t *testing.T) {
	// A ref with Name+Namespace set but empty Key is still checked at the secret level;
	// Key is irrelevant for access control — only Name and Namespace identify the secret.
	v := &AccessValidator{Client: sarAllowingClient(t, true, "")}

	refs := []telemetryv1beta1.SecretKeyRef{
		{Name: "my-secret", Namespace: "default", Key: ""},
	}
	userInfo := authenticationv1.UserInfo{Username: "alice"}

	err := v.ValidateSecretAccess(t.Context(), userInfo, refs)
	assert.NoError(t, err)
}

func TestAccessValidator_ValidateSecretAccess_EmptyRefs(t *testing.T) {
	v := &AccessValidator{Client: sarAllowingClient(t, false, "should not be called")}

	err := v.ValidateSecretAccess(t.Context(), authenticationv1.UserInfo{Username: "alice"}, nil)
	assert.NoError(t, err)
}

func TestAccessValidator_ValidateSecretAccess_DeduplicatesPerSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, authorizationv1.AddToScheme(scheme))

	callCount := 0

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if sar, ok := obj.(*authorizationv1.SubjectAccessReview); ok {
					callCount++
					sar.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}

					return nil
				}

				return cl.Create(ctx, obj, opts...)
			},
		}).
		Build()

	// Three refs, but only two distinct secrets (my-secret/default appears twice).
	refs := []telemetryv1beta1.SecretKeyRef{
		{Name: "my-secret", Namespace: "default", Key: "host"},
		{Name: "my-secret", Namespace: "default", Key: "password"},
		{Name: "other-secret", Namespace: "default", Key: "token"},
	}
	userInfo := authenticationv1.UserInfo{Username: "alice"}

	v := &AccessValidator{Client: c}
	err := v.ValidateSecretAccess(t.Context(), userInfo, refs)
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount, "expected one SAR per unique secret, not one per ref")
}

func TestAccessValidator_ConvertExtra(t *testing.T) {
	extra := map[string]authenticationv1.ExtraValue{
		"scopes": {"read", "write"},
	}

	out := convertExtra(extra)
	require.Contains(t, out, "scopes")
	assert.Equal(t, authorizationv1.ExtraValue{"read", "write"}, out["scopes"])
}

func TestAccessValidator_ConvertExtra_Nil(t *testing.T) {
	assert.Nil(t, convertExtra(nil))
}
