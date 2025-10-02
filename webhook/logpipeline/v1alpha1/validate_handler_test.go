package v1alpha1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestHandle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("should return a warning when a custom plugin is used", func(t *testing.T) {
		logPipeline := testutils.NewLogPipelineBuilder().WithName("custom-output").WithCustomOutput("Name stdout").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()

		sut := newValidateHandler(fakeClient, scheme)

		response := sut.Handle(t.Context(), admissionRequestFrom(t, logPipeline))

		require.True(t, response.Allowed)
		require.Contains(t, response.Warnings, "Logpipeline 'custom-output' uses unsupported custom filters or outputs. We recommend changing the pipeline to use supported filters or output. See the documentation: https://kyma-project.io/#/telemetry-manager/user/02-logs")
	})
}

func admissionRequestFrom(t *testing.T, logPipeline telemetryv1alpha1.LogPipeline) admission.Request {
	t.Helper()

	pipelineJSON, err := json.Marshal(logPipeline)
	if err != nil {
		t.Fatalf("failed to marshal log pipeline: %v", err)
	}

	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: pipelineJSON},
		},
	}
}
