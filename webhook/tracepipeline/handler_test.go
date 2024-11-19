package tracepipeline

import (
	"context"
	"encoding/json"
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

func TestHandle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("should add defaults for trace pipeline", func(t *testing.T) {
		tracePipeline := testutils.NewTracePipelineBuilder().WithName("default").Build()

		sut := NewDefaultingWebhookHandler(scheme)

		response := sut.Handle(context.Background(), admissionRequestFrom(t, tracePipeline))

		require.True(t, response.Allowed)
		require.Len(t, response.Patches, 1, "should have 1 patches")

		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/output/otlp/protocol",
			Value:     "grpc",
		}, "should have added default OTLP protocol")
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

func admissionRequestFrom(t *testing.T, tracePipeline telemetryv1alpha1.TracePipeline) admission.Request {
	t.Helper()

	pipelineJSON, err := json.Marshal(tracePipeline)
	if err != nil {
		t.Fatalf("failed to marshal log pipeline: %v", err)
	}

	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: pipelineJSON},
		},
	}
}
