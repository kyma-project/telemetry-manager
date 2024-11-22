package v1alpha1

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestVariableNotGloballyUnique(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	existingPipeline := testutils.NewLogPipelineBuilder().
		WithName("log-pipeline-1").
		WithVariable("foo1", "fooN", "fooNs", "foo").
		WithVariable("foo2", "fooN", "fooNs", "foo").
		Build()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&existingPipeline).Build()

	sut := NewValidatingWebhookHandler(fakeClient, scheme)

	newPipeline := testutils.NewLogPipelineBuilder().
		WithName("log-pipeline-2").
		WithVariable("foo2", "fooN", "fooNs", "foo").
		Build()

	response := sut.Handle(context.Background(), admissionRequestFrom(t, newPipeline))

	require.False(t, response.Allowed)
	require.EqualValues(t, response.Result.Code, http.StatusBadRequest)
	require.Equal(t, response.Result.Message, "variable name must be globally unique: variable 'foo2' is used in pipeline 'log-pipeline-1'")
}

func TestVariableValidator(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()

	sut := NewValidatingWebhookHandler(fakeClient, scheme)

	newPipeline := testutils.NewLogPipelineBuilder().
		WithName("log-pipeline-2").
		WithVariable("foo2", "", "", "").
		Build()

	response := sut.Handle(context.Background(), admissionRequestFrom(t, newPipeline))

	require.False(t, response.Allowed)
	require.EqualValues(t, response.Result.Code, http.StatusBadRequest)
	require.Equal(t, response.Result.Message, "mandatory field variable name or secretKeyRef name or secretKeyRef namespace or secretKeyRef key cannot be empty")
}
