package test

import (
	"context"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ContextWithAdmissionRequest returns a context carrying a minimal admission.Request
// with the given username. This is required by webhook validators that call
// admission.RequestFromContext to perform SubjectAccessReview checks.
func ContextWithAdmissionRequest(t *testing.T, username string) context.Context {
	t.Helper()

	req := admission.Request{
		UserInfo: authenticationv1.UserInfo{
			Username: username,
			Groups:   []string{"system:authenticated"},
		},
	}

	return admission.NewContextWithRequest(t.Context(), req)
}
