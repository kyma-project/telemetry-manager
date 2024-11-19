package logpipeline

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestMutatingHandle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("should add defaults for log pipeline", func(t *testing.T) {
		logPipeline := testutils.NewLogPipelineBuilder().WithName("default").WithDropLabels(false).Build()

		sut := NewDefaultingWebhookHandler(scheme)

		response := sut.Handle(context.Background(), admissionRequestFrom(t, logPipeline))

		require.True(t, response.Allowed)
		require.Len(t, response.Patches, 2, "should have 2 patches")

		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/application/enabled",
			Value:     true,
		}, "should have added default application input enabled true")

		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/application/keepOriginalBody",
			Value:     true,
		}, "should have added default application input keepBodyEnabled true")
	})

	t.Run("should not add defaults if application input is already set", func(t *testing.T) {
		logPipeline := testutils.NewLogPipelineBuilder().WithName("default").WithApplicationInputDisabled().Build()

		sut := NewDefaultingWebhookHandler(scheme)

		response := sut.Handle(context.Background(), admissionRequestFrom(t, logPipeline))

		require.True(t, response.Allowed)
		require.Len(t, response.Patches, 1, "should have 1 patch")
		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/application/keepOriginalBody",
			Value:     true,
		}, "should have added default application input keepBodyEnabled true")
	})

	t.Run("should handle decoding error", func(t *testing.T) {
		sut := NewDefaultingWebhookHandler(scheme)

		invalidRequest := admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{Raw: []byte("invalid json")},
			},
		}

		response := sut.Handle(context.Background(), invalidRequest)

		require.False(t, response.Allowed)
		require.Equal(t, int32(http.StatusBadRequest), response.Result.Code)
	})
}
