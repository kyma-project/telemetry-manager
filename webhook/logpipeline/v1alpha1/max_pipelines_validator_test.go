package v1alpha1

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestValidateFirstPipeline(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()

	sut := NewValidatingWebhookHandler(fakeClient, scheme)

	newPipeline := testutils.NewLogPipelineBuilder().Build()

	response := sut.Handle(context.Background(), admissionRequestFrom(t, newPipeline))

	require.True(t, response.Allowed)
}

func TestValidateLimitNotExceeded(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	var existingPipelines []client.Object

	for i := range 4 {
		p := testutils.NewLogPipelineBuilder().WithName(strconv.Itoa(i)).Build()
		existingPipelines = append(existingPipelines, &p)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingPipelines...).Build()

	sut := NewValidatingWebhookHandler(fakeClient, scheme)

	newPipeline := testutils.NewLogPipelineBuilder().Build()

	response := sut.Handle(context.Background(), admissionRequestFrom(t, newPipeline))

	require.True(t, response.Allowed)
}

func TestValidateLimitExceeded(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	var existingPipelines []client.Object

	for i := range 5 {
		p := testutils.NewLogPipelineBuilder().WithName(strconv.Itoa(i)).Build()
		existingPipelines = append(existingPipelines, &p)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingPipelines...).Build()

	sut := NewValidatingWebhookHandler(fakeClient, scheme)

	newPipeline := testutils.NewLogPipelineBuilder().Build()

	response := sut.Handle(context.Background(), admissionRequestFrom(t, newPipeline))

	require.False(t, response.Allowed)
	require.EqualValues(t, response.Result.Code, http.StatusBadRequest)
	require.Equal(t, response.Result.Message, "the maximum number of log pipelines is 5")
}

func TestValidateUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	existingPipeline := testutils.NewLogPipelineBuilder().WithName("foo").Build()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&existingPipeline).Build()

	sut := NewValidatingWebhookHandler(fakeClient, scheme)

	response := sut.Handle(context.Background(), admissionRequestFrom(t, existingPipeline))

	require.True(t, response.Allowed)
}
