package logpipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry/mocks"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestGetOutputType(t *testing.T) {
	tests := []struct {
		name     string
		pipeline telemetryv1beta1.LogPipeline
		want     logpipelineutils.Mode
	}{
		{
			name:     "OTLP output returns OTel mode",
			pipeline: testutils.NewLogPipelineBuilder().WithOTLPOutput().Build(),
			want:     logpipelineutils.OTel,
		},
		{
			name:     "HTTP output returns FluentBit mode",
			pipeline: testutils.NewLogPipelineBuilder().WithHTTPOutput().Build(),
			want:     logpipelineutils.FluentBit,
		},
		{
			name:     "Custom output returns FluentBit mode",
			pipeline: testutils.NewLogPipelineBuilder().WithCustomOutput("custom").Build(),
			want:     logpipelineutils.FluentBit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logpipelineutils.GetOutputType(&tt.pipeline)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGetPipelinesForType(t *testing.T) {
	otelPipeline := testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()
	fluentbitPipeline1 := testutils.NewLogPipelineBuilder().WithHTTPOutput().Build()
	fluentbitPipeline2 := testutils.NewLogPipelineBuilder().WithHTTPOutput().Build()

	fakeClient := newTestClient(t, &otelPipeline, &fluentbitPipeline1, &fluentbitPipeline2)

	got, err := logpipelineutils.GetPipelinesForType(t.Context(), fakeClient, logpipelineutils.OTel)
	require.NoError(t, err)
	require.ElementsMatch(t, pipelineNames(got), []string{otelPipeline.Name})

	got, err = logpipelineutils.GetPipelinesForType(t.Context(), fakeClient, logpipelineutils.FluentBit)
	require.NoError(t, err)
	require.ElementsMatch(t, pipelineNames(got), []string{fluentbitPipeline1.Name, fluentbitPipeline2.Name})
}

func pipelineNames(pipelines []telemetryv1beta1.LogPipeline) []string {
	names := make([]string, len(pipelines))
	for i, p := range pipelines {
		names[i] = p.Name
	}

	return names
}

var _ LogPipelineReconciler = &stubs.ReconcilerStub{}

func TestRegisterAndCallRegisteredReconciler(t *testing.T) {
	otelPipeline := testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()
	unsupportedPipeline := testutils.NewLogPipelineBuilder().WithHTTPOutput().Build()

	fakeClient := newTestClient(t, &otelPipeline, &unsupportedPipeline)

	otelReconciler := &stubs.ReconcilerStub{
		OutputType: logpipelineutils.OTel,
		Result:     nil,
	}

	rec := newTestReconciler(fakeClient, WithReconcilers(otelReconciler))

	result := reconcile(t, rec, otelPipeline.Name)
	require.NoError(t, result.err)
	require.NotNil(t, result.result)

	result = reconcile(t, rec, unsupportedPipeline.Name)
	require.ErrorIs(t, result.err, ErrUnsupportedOutputType)
	require.NotNil(t, result.result)
}

func TestReconcile_PausedOverride(t *testing.T) {
	fakeClient := newTestClient(t)

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(&overrides.Config{
		Logging: overrides.LoggingConfig{Paused: true},
	}, nil)

	rec := newTestReconciler(fakeClient, WithOverridesHandler(overridesHandler))

	result := reconcile(t, rec, "nonexistent-pipeline")
	require.NoError(t, result.err)
	require.Equal(t, ctrl.Result{}, result.result)
}

func TestReconcile_MissingLogPipeline(t *testing.T) {
	fakeClient := newTestClient(t)

	rec := newTestReconciler(fakeClient)

	result := reconcile(t, rec, "nonexistent-pipeline")
	require.NoError(t, result.err)
	require.Equal(t, ctrl.Result{}, result.result)
}

func TestReconcile_UnsupportedOutputType(t *testing.T) {
	unsupportedPipeline := testutils.NewLogPipelineBuilder().WithCustomOutput("custom").Build()

	fakeClient := newTestClient(t, &unsupportedPipeline)

	rec := newTestReconciler(fakeClient)

	result := reconcile(t, rec, unsupportedPipeline.Name)
	require.ErrorIs(t, result.err, ErrUnsupportedOutputType)
	require.Equal(t, ctrl.Result{}, result.result)
}

func TestReconcile_LoadingOverridesFails(t *testing.T) {
	fakeClient := newTestClient(t)

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(nil, fmt.Errorf("error loading overrides"))

	rec := newTestReconciler(fakeClient, WithOverridesHandler(overridesHandler))

	result := reconcile(t, rec, "nonexistent-pipeline")
	require.Error(t, result.err)
	require.Equal(t, ctrl.Result{}, result.result)
}

// TestPipelineDeletionCleanup verifies ConfigMap cleanup when pipeline is deleted
func TestPipelineDeletionCleanup(t *testing.T) {
	tests := []struct {
		name                           string
		removeFromWatchersError        error
		removeReferenceError           error
		expectRemoveFromWatchersCalled bool
		expectRemoveReferenceCalled    bool
		expectError                    bool
	}{
		{
			name:                           "successful deletion cleanup",
			expectRemoveFromWatchersCalled: true,
			expectRemoveReferenceCalled:    true,
			expectError:                    false,
		},
		{
			name:                           "RemoveFromWatchers fails",
			removeFromWatchersError:        assert.AnError,
			expectRemoveFromWatchersCalled: true,
			expectRemoveReferenceCalled:    false,
			expectError:                    true,
		},
		{
			name:                           "RemoveLogPipelineReference fails",
			removeReferenceError:           assert.AnError,
			expectRemoveFromWatchersCalled: true,
			expectRemoveReferenceCalled:    true,
			expectError:                    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := newTestClient(t)

			secretWatcher := stubs.NewSecretWatcher(tt.removeFromWatchersError)

			// Create reconciler with WithGlobals option and mocked secret watcher
			cfg := config.NewGlobal(config.WithTargetNamespace("kyma-system"))
			rec := newTestReconciler(
				fakeClient,
				WithGlobals(cfg),
				WithSecretWatcher(secretWatcher),
			)

			// Mock ConfigMap operations for reference removal
			if tt.removeReferenceError != nil {
				rec.Client = &removeReferenceErrorClient{Client: fakeClient, err: tt.removeReferenceError}
			}

			result := reconcile(t, rec, "deleted-pipeline")

			if tt.expectError {
				require.Error(t, result.err)
			} else {
				require.NoError(t, result.err)
			}
		})
	}
}

// removeReferenceErrorClient simulates errors when updating OTLP Gateway Pipelines Sync ConfigMap
type removeReferenceErrorClient struct {
	client.Client

	err error
}

func (c *removeReferenceErrorClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	// Return error for OTLP Gateway Pipelines Sync ConfigMap to simulate RemoveLogPipelineReference failure
	if cm, ok := obj.(*corev1.ConfigMap); ok && key.Name == names.OTLPGatewayPipelinesSyncConfigMap {
		_ = cm
		return c.err
	}

	return c.Client.Get(ctx, key, obj, opts...)
}

// TestGetPipelineErrors verifies error handling during pipeline retrieval
func TestGetPipelineErrors(t *testing.T) {
	tests := []struct {
		name        string
		getError    error
		expectError bool
	}{
		{
			name:        "API server error during Get",
			getError:    assert.AnError,
			expectError: true,
		},
		{
			name:        "Context canceled during Get",
			getError:    context.Canceled,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &errorOnGetClient{err: tt.getError}

			rec := newTestReconciler(fakeClient)

			result := reconcile(t, rec, "test-pipeline")

			if tt.expectError {
				require.Error(t, result.err)
			} else {
				require.NoError(t, result.err)
			}
		})
	}
}

// errorOnGetClient simulates API errors during Get operations
type errorOnGetClient struct {
	client.Client

	err error
}

func (c *errorOnGetClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*telemetryv1beta1.LogPipeline); ok {
		return c.err
	}

	return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
}

// TestLockAcquisitionFailures verifies handling of pipeline lock errors
func TestLockAcquisitionFailures(t *testing.T) {
	tests := []struct {
		name            string
		lockError       error
		expectError     bool
		expectReconcile bool
	}{
		{
			name:            "Max pipelines exceeded",
			lockError:       resourcelock.ErrMaxPipelinesExceeded,
			expectError:     false, // Handled gracefully
			expectReconcile: false, // Reconciliation skipped
		},
		{
			name:            "Lock acquisition API error",
			lockError:       assert.AnError,
			expectError:     true,
			expectReconcile: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()
			fakeClient := newTestClient(t, &pipeline)

			pipelineSync := &logpipelinemocks.PipelineSyncer{}
			pipelineSync.On("TryAcquireLock", mock.Anything, mock.Anything).Return(tt.lockError)

			rec := newTestReconciler(fakeClient, WithPipelineSyncer(pipelineSync))

			result := reconcile(t, rec, pipeline.Name)

			if tt.expectError {
				require.Error(t, result.err)
			} else {
				require.NoError(t, result.err)
			}
		})
	}
}
