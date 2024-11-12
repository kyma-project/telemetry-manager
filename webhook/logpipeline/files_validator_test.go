package logpipeline

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestDuplicateFileName(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	existingPipeline := testutils.NewLogPipelineBuilder().WithName("foo").WithFile("f1.json", "").Build()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&existingPipeline).Build()

	sut := NewValidatingWebhookHandler(fakeClient, scheme)

	newPipeline := testutils.NewLogPipelineBuilder().WithName("bar").WithFile("f1.json", "").Build()

	response := sut.Handle(context.Background(), admissionRequestFrom(t, newPipeline))

	require.False(t, response.Allowed)
	require.EqualValues(t, response.Result.Code, http.StatusBadRequest)
	require.Equal(t, response.Result.Message, "filename 'f1.json' is already being used in the logPipeline 'foo'")
}

func TestDuplicateFileNameInSamePipeline(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()

	sut := NewValidatingWebhookHandler(fakeClient, scheme)

	newPipeline := testutils.NewLogPipelineBuilder().WithName("foo").WithFile("f1.json", "").WithFile("f1.json", "").Build()

	response := sut.Handle(context.Background(), admissionRequestFrom(t, newPipeline))

	require.False(t, response.Allowed)
	require.EqualValues(t, response.Result.Code, http.StatusBadRequest)
	require.Equal(t, response.Result.Message, "duplicate file names detected please review your pipeline")
}

func TestValidateUpdatePipeline(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	existingPipeline := testutils.NewLogPipelineBuilder().WithName("foo").WithFile("f1.json", "").Build()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&existingPipeline).Build()

	sut := NewValidatingWebhookHandler(fakeClient, scheme)

	newPipeline := testutils.NewLogPipelineBuilder().WithName("foo").WithFile("f1.json", "").Build()

	response := sut.Handle(context.Background(), admissionRequestFrom(t, newPipeline))

	require.True(t, response.Allowed)
}
