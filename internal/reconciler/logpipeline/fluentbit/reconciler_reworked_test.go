package fluentbit

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	commonStatusMocks "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/mocks"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/fluentbit/mocks"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func TestMaxPipelines(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	pipeline := testutils.NewLogPipelineBuilder().
		WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
		WithCustomFilter("Name grep").
		Build()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

	// Create reconciler that simulates max pipelines exceeded
	reconciler := createMaxPipelinesTestReconciler(fakeClient, pipeline)

	// Execute
	var pl telemetryv1alpha1.LogPipeline
	require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &pl))
	err := reconciler.Reconcile(t.Context(), &pl)
	require.NoError(t, err)

	// Verify
	var updatedPipeline telemetryv1alpha1.LogPipeline
	_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

	requireHasStatusCondition(t, updatedPipeline,
		conditions.TypeConfigurationGenerated,
		metav1.ConditionFalse,
		conditions.ReasonMaxPipelinesExceeded,
		"Maximum pipeline count limit exceeded",
	)

	requireHasStatusCondition(t, updatedPipeline,
		conditions.TypeFlowHealthy,
		metav1.ConditionFalse,
		conditions.ReasonSelfMonConfigNotGenerated,
		"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
	)
}

func TestTLSConditions(t *testing.T) {
	tests := []struct {
		name                  string
		tlsCertErr            error
		expectedStatus        metav1.ConditionStatus
		expectedReason        string
		expectedMessage       string
		expectAgentConfigured bool
	}{
		{
			name:            "cert expired",
			tlsCertErr:      &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC)},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonTLSCertificateExpired,
			expectedMessage: "TLS certificate expired on 2020-11-01",
		},
		{
			name:                  "cert about to expire",
			tlsCertErr:            &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC)},
			expectedStatus:        metav1.ConditionTrue,
			expectedReason:        conditions.ReasonTLSCertificateAboutToExpire,
			expectedMessage:       "TLS certificate is about to expire, configured certificate is valid until 2024-11-01",
			expectAgentConfigured: true,
		},
		{
			name:            "ca expired",
			tlsCertErr:      &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonTLSCertificateExpired,
			expectedMessage: "TLS CA certificate expired on 2020-11-01",
		},
		{
			name:                  "ca about to expire",
			tlsCertErr:            &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
			expectedStatus:        metav1.ConditionTrue,
			expectedReason:        conditions.ReasonTLSCertificateAboutToExpire,
			expectedMessage:       "TLS CA certificate is about to expire, configured certificate is valid until 2024-11-01",
			expectAgentConfigured: true,
		},
		{
			name:            "cert decode failed",
			tlsCertErr:      tlscert.ErrCertDecodeFailed,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonTLSConfigurationInvalid,
			expectedMessage: "TLS configuration invalid: failed to decode PEM block containing certificate",
		},
		{
			name:            "key decode failed",
			tlsCertErr:      tlscert.ErrKeyDecodeFailed,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonTLSConfigurationInvalid,
			expectedMessage: "TLS configuration invalid: failed to decode PEM block containing private key",
		},
		{
			name:            "cert parse failed",
			tlsCertErr:      tlscert.ErrCertParseFailed,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonTLSConfigurationInvalid,
			expectedMessage: "TLS configuration invalid: failed to parse certificate",
		},
		{
			name:            "cert and key mismatch",
			tlsCertErr:      tlscert.ErrInvalidCertificateKeyPair,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonTLSConfigurationInvalid,
			expectedMessage: "TLS configuration invalid: certificate and private key do not match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			pipeline := testutils.NewLogPipelineBuilder().
				WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
				WithHTTPOutput(testutils.HTTPClientTLSFromString("ca", "fooCert", "fooKey")).
				Build()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

			// Create reconciler with specific TLS cert validation error
			reconciler := createTLSTestReconciler(fakeClient, pipeline, tt.tlsCertErr)

			// Execute
			var pl telemetryv1alpha1.LogPipeline
			require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &pl))
			err := reconciler.Reconcile(t.Context(), &pl)
			require.NoError(t, err)

			// Verify
			var updatedPipeline telemetryv1alpha1.LogPipeline
			_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

			requireHasStatusCondition(t, updatedPipeline,
				conditions.TypeConfigurationGenerated,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMessage,
			)

			if tt.expectedStatus == metav1.ConditionFalse {
				requireHasStatusCondition(t, updatedPipeline,
					conditions.TypeFlowHealthy,
					metav1.ConditionFalse,
					conditions.ReasonSelfMonConfigNotGenerated,
					"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
				)
			}
		})
	}
}

func TestPodErrorConditions(t *testing.T) {
	tests := []struct {
		name            string
		probeErr        error
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "pod is OOM",
			probeErr:        &workloadstatus.PodIsPendingError{ContainerName: "foo", Reason: "OOMKilled", Message: ""},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonAgentNotReady,
			expectedMessage: "Pod is in the pending state because container: foo is not running due to: OOMKilled. Please check the container: foo logs.",
		},
		{
			name:            "pod is CrashLoop",
			probeErr:        &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonAgentNotReady,
			expectedMessage: "Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
		},
		{
			name:            "no Pods deployed",
			probeErr:        workloadstatus.ErrNoPodsDeployed,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonAgentNotReady,
			expectedMessage: "No Pods deployed",
		},
		{
			name:            "fluent bit rollout in progress",
			probeErr:        &workloadstatus.RolloutInProgressError{},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  conditions.ReasonRolloutInProgress,
			expectedMessage: "Pods are being started/updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

			// Create reconciler with specific prober error
			reconciler := createPodErrorTestReconciler(fakeClient, pipeline, tt.probeErr)

			// Execute
			var pl telemetryv1alpha1.LogPipeline
			require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &pl))
			err := reconciler.Reconcile(t.Context(), &pl)
			require.NoError(t, err)

			// Verify
			var updatedPipeline telemetryv1alpha1.LogPipeline
			_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
			cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeAgentHealthy)
			require.Equal(t, tt.expectedStatus, cond.Status)
			require.Equal(t, tt.expectedReason, cond.Reason)
			require.Equal(t, tt.expectedMessage, cond.Message)
		})
	}
}

func TestUnsupportedMode(t *testing.T) {
	tests := []struct {
		name            string
		pipelineBuilder func() telemetryv1alpha1.LogPipeline
		expectedMode    bool
	}{
		{
			name: "should set status UnsupportedMode true if contains custom plugin",
			pipelineBuilder: func() telemetryv1alpha1.LogPipeline {
				return testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					WithCustomFilter("Name grep").
					Build()
			},
			expectedMode: true,
		},
		{
			name: "should set status UnsupportedMode false if does not contains custom plugin",
			pipelineBuilder: func() telemetryv1alpha1.LogPipeline {
				return testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					Build()
			},
			expectedMode: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			pipeline := tt.pipelineBuilder()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

			// Create reconciler with mocks
			reconciler := createTestReconciler(fakeClient, pipeline)

			// Execute
			var pl telemetryv1alpha1.LogPipeline
			require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &pl))
			err := reconciler.Reconcile(t.Context(), &pl)
			require.NoError(t, err)

			// Verify
			var updatedPipeline telemetryv1alpha1.LogPipeline
			_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
			require.Equal(t, tt.expectedMode, *updatedPipeline.Status.UnsupportedMode)
		})
	}
}

func TestReferencedSecret(t *testing.T) {
	tests := []struct {
		name               string
		setupPipeline      func() (telemetryv1alpha1.LogPipeline, []client.Object)
		secretRefError     error
		expectedError      error
		expectedConditions []expectedCondition
	}{
		{
			name: "a request to the Kubernetes API server has failed when validating the secret references",
			setupPipeline: func() (telemetryv1alpha1.LogPipeline, []client.Object) {
				pipeline := testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					Build()
				return pipeline, []client.Object{&pipeline}
			},
			secretRefError: &errortypes.APIRequestFailedError{Err: errors.New("failed to get secret: server error")},
			expectedError:  errors.New("failed to get secret: server error"),
			expectedConditions: []expectedCondition{
				{
					conditionType: conditions.TypeConfigurationGenerated,
					status:        metav1.ConditionFalse,
					reason:        conditions.ReasonValidationFailed,
					message:       "Pipeline validation failed due to an error from the Kubernetes API server",
				},
				{
					conditionType: conditions.TypeFlowHealthy,
					status:        metav1.ConditionFalse,
					reason:        conditions.ReasonSelfMonConfigNotGenerated,
					message:       "No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
				},
			},
		},
		{
			name: "referenced secret missing",
			setupPipeline: func() (telemetryv1alpha1.LogPipeline, []client.Object) {
				pipeline := testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					Build()
				return pipeline, []client.Object{&pipeline}
			},
			secretRefError: fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound),
			expectedConditions: []expectedCondition{
				{
					conditionType: conditions.TypeConfigurationGenerated,
					status:        metav1.ConditionFalse,
					reason:        conditions.ReasonReferencedSecretMissing,
					message:       "One or more referenced Secrets are missing: Secret 'some-secret' of Namespace 'some-namespace'",
				},
				{
					conditionType: conditions.TypeFlowHealthy,
					status:        metav1.ConditionFalse,
					reason:        conditions.ReasonSelfMonConfigNotGenerated,
					message:       "No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
				},
			},
		},
		{
			name: "referenced secret exists",
			setupPipeline: func() (telemetryv1alpha1.LogPipeline, []client.Object) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-secret",
						Namespace: "some-namespace",
					},
					Data: map[string][]byte{"host": nil},
				}
				pipeline := testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					WithHTTPOutput(testutils.HTTPHostFromSecret("some-secret", "some-namespace", "host")).
					Build()
				return pipeline, []client.Object{&pipeline, secret}
			},
			secretRefError: nil,
			expectedConditions: []expectedCondition{
				{
					conditionType: conditions.TypeConfigurationGenerated,
					status:        metav1.ConditionTrue,
					reason:        conditions.ReasonAgentConfigured,
					message:       "LogPipeline specification is successfully applied to the configuration of Log agent",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			pipeline, objects := tt.setupPipeline()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithStatusSubresource(&pipeline).Build()

			// Create reconciler with specific secret reference validation error
			reconciler := createSecretRefTestReconciler(fakeClient, pipeline, tt.secretRefError)

			// Execute
			var pl telemetryv1alpha1.LogPipeline
			require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &pl))
			err := reconciler.Reconcile(t.Context(), &pl)

			// Check expected error
			if tt.expectedError != nil {
				require.Error(t, err)
				// For APIRequestFailedError, we need to check the wrapped error
				if apiErr, ok := tt.secretRefError.(*errortypes.APIRequestFailedError); ok {
					require.True(t, errors.Is(err, apiErr.Err))
				} else {
					require.True(t, errors.Is(err, tt.expectedError))
				}
			} else {
				require.NoError(t, err)
			}

			// Verify conditions
			var updatedPipeline telemetryv1alpha1.LogPipeline
			_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

			for _, expected := range tt.expectedConditions {
				requireHasStatusCondition(t, updatedPipeline,
					expected.conditionType,
					expected.status,
					expected.reason,
					expected.message,
				)
			}
		})
	}
}

func TestLogAgent(t *testing.T) {
	tests := []struct {
		name                string
		proberError         error
		errorConverter      interface{}
		expectedCondition   metav1.ConditionStatus
		expectedReason      string
		expectedMessage     string
		setupErrorConverter func(*commonStatusMocks.ErrorToMessageConverter)
	}{
		{
			name:              "log agent is not ready",
			proberError:       workloadstatus.ErrDaemonSetNotFound,
			errorConverter:    &commonStatusMocks.ErrorToMessageConverter{},
			expectedCondition: metav1.ConditionFalse,
			expectedReason:    conditions.ReasonAgentNotReady,
			expectedMessage:   workloadstatus.ErrDaemonSetNotFound.Error(),
			setupErrorConverter: func(stub *commonStatusMocks.ErrorToMessageConverter) {
				stub.On("Convert", mock.Anything).Return("DaemonSet is not yet created")
			},
		},
		{
			name:              "log agent is ready",
			proberError:       nil,
			errorConverter:    &commonStatusMocks.ErrorToMessageConverter{},
			expectedCondition: metav1.ConditionTrue,
			expectedReason:    conditions.ReasonAgentReady,
			expectedMessage:   "Log agent DaemonSet is ready",
		},
		{
			name:              "log agent prober fails",
			proberError:       workloadstatus.ErrDaemonSetFetching,
			errorConverter:    &conditions.ErrorToMessageConverter{},
			expectedCondition: metav1.ConditionFalse,
			expectedReason:    conditions.ReasonAgentNotReady,
			expectedMessage:   "Failed to get DaemonSet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

			// Create reconciler with specific prober error
			reconciler := createLogAgentTestReconciler(fakeClient, pipeline, tt.proberError, tt.errorConverter, tt.setupErrorConverter)

			// Execute
			var pl telemetryv1alpha1.LogPipeline
			require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &pl))
			err := reconciler.Reconcile(t.Context(), &pl)
			require.NoError(t, err)

			// Verify
			var updatedPipeline telemetryv1alpha1.LogPipeline
			_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
			requireHasStatusCondition(t, updatedPipeline,
				conditions.TypeAgentHealthy,
				tt.expectedCondition,
				tt.expectedReason,
				tt.expectedMessage,
			)
		})
	}
}

func TestFlowHealthy(t *testing.T) {
	tests := []struct {
		name            string
		probe           prober.FluentBitProbeResult
		probeErr        error
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "prober fails",
			probeErr:        assert.AnError,
			expectedStatus:  metav1.ConditionUnknown,
			expectedReason:  conditions.ReasonSelfMonAgentProbingFailed,
			expectedMessage: "Could not determine the health of the telemetry flow because the self monitor probing of agent failed",
		},
		{
			name: "healthy",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
			},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  conditions.ReasonSelfMonFlowHealthy,
			expectedMessage: "No problems detected in the telemetry flow",
		},
		{
			name: "buffer filling up",
			probe: prober.FluentBitProbeResult{
				BufferFillingUp: true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonAgentBufferFillingUp,
			expectedMessage: "Buffer nearing capacity. Incoming log rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=agent-buffer-filling-up",
		},
		{
			name: "no logs delivered",
			probe: prober.FluentBitProbeResult{
				NoLogsDelivered: true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonAgentNoLogsDelivered,
			expectedMessage: "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
		},
		{
			name: "no logs delivered shadows other problems",
			probe: prober.FluentBitProbeResult{
				NoLogsDelivered: true,
				BufferFillingUp: true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonAgentNoLogsDelivered,
			expectedMessage: "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
		},
		{
			name: "some data dropped",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonAgentSomeDataDropped,
			expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=not-all-logs-arrive-at-the-backend",
		},
		{
			name: "some data dropped shadows other problems",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				BufferFillingUp:     true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonAgentSomeDataDropped,
			expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=not-all-logs-arrive-at-the-backend",
		},
		{
			name: "all data dropped",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonAgentAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
		},
		{
			name: "all data dropped shadows other problems",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{
					AllDataDropped:  true,
					SomeDataDropped: true,
				},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonAgentAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

			// Create reconciler with specific flow health prober settings
			reconciler := createFlowHealthTestReconciler(fakeClient, pipeline, tt.probe, tt.probeErr)

			// Execute
			var pl telemetryv1alpha1.LogPipeline
			require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &pl))
			err := reconciler.Reconcile(t.Context(), &pl)
			require.NoError(t, err)

			// Verify
			var updatedPipeline telemetryv1alpha1.LogPipeline
			_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
			requireHasStatusCondition(t, updatedPipeline,
				conditions.TypeFlowHealthy,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMessage,
			)
		})
	}
}

func createTestReconciler(fakeClient client.Client, pipeline telemetryv1alpha1.LogPipeline) *Reconciler {
	globals := config.NewGlobal(config.WithNamespace("default"))

	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.FluentBitProbeResult{}, nil)

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	validator := &Validator{
		EndpointValidator:  stubs.NewEndpointValidator(nil),
		TLSCertValidator:   stubs.NewTLSCertValidator(nil),
		SecretRefValidator: stubs.NewSecretRefValidator(nil),
		PipelineLock:       pipelineLock,
	}

	return New(
		globals,
		fakeClient,
		agentConfigBuilder,
		agentApplierDeleter,
		commonStatusStubs.NewDaemonSetProber(nil),
		flowHealthProber,
		&stubs.IstioStatusChecker{IsActive: false},
		pipelineLock,
		validator,
		&commonStatusMocks.ErrorToMessageConverter{},
	)
}

func createLogAgentTestReconciler(fakeClient client.Client, pipeline telemetryv1alpha1.LogPipeline, proberError error, errorConverter interface{}, setupErrorConverter func(*commonStatusMocks.ErrorToMessageConverter)) *Reconciler {
	globals := config.NewGlobal(config.WithNamespace("default"))

	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	proberStub := commonStatusStubs.NewDaemonSetProber(proberError)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.FluentBitProbeResult{}, nil)

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	validator := &Validator{
		EndpointValidator:  stubs.NewEndpointValidator(nil),
		TLSCertValidator:   stubs.NewTLSCertValidator(nil),
		SecretRefValidator: stubs.NewSecretRefValidator(nil),
		PipelineLock:       pipelineLock,
	}

	// Convert errorConverter to the proper type and setup if needed
	if setupErrorConverter != nil {
		if mockConverter, ok := errorConverter.(*commonStatusMocks.ErrorToMessageConverter); ok {
			setupErrorConverter(mockConverter)
		}
	}

	// Type assert to the interface expected by the New function
	var converter ErrorToMessageConverter
	switch v := errorConverter.(type) {
	case *commonStatusMocks.ErrorToMessageConverter:
		converter = v
	case *conditions.ErrorToMessageConverter:
		converter = v
	default:
		converter = &commonStatusMocks.ErrorToMessageConverter{}
	}

	return New(
		globals,
		fakeClient,
		agentConfigBuilder,
		agentApplierDeleter,
		proberStub,
		flowHealthProber,
		&stubs.IstioStatusChecker{IsActive: false},
		pipelineLock,
		validator,
		converter,
	)
}

type expectedCondition struct {
	conditionType string
	status        metav1.ConditionStatus
	reason        string
	message       string
}

func createSecretRefTestReconciler(fakeClient client.Client, pipeline telemetryv1alpha1.LogPipeline, secretRefError error) *Reconciler {
	globals := config.NewGlobal(config.WithNamespace("default"))

	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	proberStub := commonStatusStubs.NewDaemonSetProber(nil)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.FluentBitProbeResult{}, nil)

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	validator := &Validator{
		EndpointValidator:  stubs.NewEndpointValidator(nil),
		TLSCertValidator:   stubs.NewTLSCertValidator(nil),
		SecretRefValidator: stubs.NewSecretRefValidator(secretRefError),
		PipelineLock:       pipelineLock,
	}

	errToMsgStub := &commonStatusMocks.ErrorToMessageConverter{}
	errToMsgStub.On("Convert", mock.Anything).Return("")

	return New(
		globals,
		fakeClient,
		agentConfigBuilder,
		agentApplierDeleter,
		proberStub,
		flowHealthProber,
		&stubs.IstioStatusChecker{IsActive: false},
		pipelineLock,
		validator,
		errToMsgStub,
	)
}

func createFlowHealthTestReconciler(fakeClient client.Client, pipeline telemetryv1alpha1.LogPipeline, probe prober.FluentBitProbeResult, probeErr error) *Reconciler {
	globals := config.NewGlobal(config.WithNamespace("default"))

	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	proberStub := commonStatusStubs.NewDaemonSetProber(nil)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(probe, probeErr)

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	validator := &Validator{
		EndpointValidator:  stubs.NewEndpointValidator(nil),
		TLSCertValidator:   stubs.NewTLSCertValidator(nil),
		SecretRefValidator: stubs.NewSecretRefValidator(nil),
		PipelineLock:       pipelineLock,
	}

	errToMsgStub := &commonStatusMocks.ErrorToMessageConverter{}
	errToMsgStub.On("Convert", mock.Anything).Return("")

	return New(
		globals,
		fakeClient,
		agentConfigBuilder,
		agentApplierDeleter,
		proberStub,
		flowHealthProber,
		&stubs.IstioStatusChecker{IsActive: false},
		pipelineLock,
		validator,
		errToMsgStub,
	)
}

func createPodErrorTestReconciler(fakeClient client.Client, pipeline telemetryv1alpha1.LogPipeline, probeErr error) *Reconciler {
	globals := config.NewGlobal(config.WithNamespace("default"))

	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	proberStub := commonStatusStubs.NewDaemonSetProber(probeErr)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.FluentBitProbeResult{}, nil)

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	validator := &Validator{
		EndpointValidator:  stubs.NewEndpointValidator(nil),
		TLSCertValidator:   stubs.NewTLSCertValidator(nil),
		SecretRefValidator: stubs.NewSecretRefValidator(nil),
		PipelineLock:       pipelineLock,
	}

	errToMsgStub := &conditions.ErrorToMessageConverter{}

	return New(
		globals,
		fakeClient,
		agentConfigBuilder,
		agentApplierDeleter,
		proberStub,
		flowHealthProber,
		&stubs.IstioStatusChecker{IsActive: false},
		pipelineLock,
		validator,
		errToMsgStub,
	)
}

func createTLSTestReconciler(fakeClient client.Client, pipeline telemetryv1alpha1.LogPipeline, tlsCertErr error) *Reconciler {
	globals := config.NewGlobal(config.WithNamespace("default"))

	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	proberStub := commonStatusStubs.NewDaemonSetProber(nil)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.FluentBitProbeResult{}, nil)

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	validator := &Validator{
		EndpointValidator:  stubs.NewEndpointValidator(nil),
		TLSCertValidator:   stubs.NewTLSCertValidator(tlsCertErr),
		SecretRefValidator: stubs.NewSecretRefValidator(nil),
		PipelineLock:       pipelineLock,
	}

	errToMsgStub := &commonStatusMocks.ErrorToMessageConverter{}
	errToMsgStub.On("Convert", mock.Anything).Return("")

	return New(
		globals,
		fakeClient,
		agentConfigBuilder,
		agentApplierDeleter,
		proberStub,
		flowHealthProber,
		&stubs.IstioStatusChecker{IsActive: false},
		pipelineLock,
		validator,
		errToMsgStub,
	)
}

func createMaxPipelinesTestReconciler(fakeClient client.Client, pipeline telemetryv1alpha1.LogPipeline) *Reconciler {
	globals := config.NewGlobal(config.WithNamespace("default"))

	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	proberStub := commonStatusStubs.NewDaemonSetProber(nil)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.FluentBitProbeResult{}, nil)

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)

	validator := &Validator{
		EndpointValidator:  stubs.NewEndpointValidator(nil),
		TLSCertValidator:   stubs.NewTLSCertValidator(nil),
		SecretRefValidator: stubs.NewSecretRefValidator(nil),
		PipelineLock:       pipelineLock,
	}

	errToMsgStub := &commonStatusMocks.ErrorToMessageConverter{}

	return New(
		globals,
		fakeClient,
		agentConfigBuilder,
		agentApplierDeleter,
		proberStub,
		flowHealthProber,
		&stubs.IstioStatusChecker{IsActive: false},
		pipelineLock,
		validator,
		errToMsgStub,
	)
}
