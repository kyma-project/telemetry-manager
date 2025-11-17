package fluentbit

import (
	"context"
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
	logpipelinefluentbitmocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/fluentbit/mocks"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func TestAppInput(t *testing.T) {
	pipeline := testutils.NewLogPipelineBuilder().WithApplicationInput(false).Build()
	testClient := newTestClient(t, &pipeline)
	reconciler := newTestReconciler(testClient)

	result := reconcileAndGet(t, testClient, reconciler, pipeline.Name)
	require.NoError(t, result.err)
}

func TestMaxPipelines(t *testing.T) {
	pipeline := testutils.NewLogPipelineBuilder().
		WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
		WithCustomFilter("Name grep").
		Build()
	testClient := newTestClient(t, &pipeline)
	reconciler := newMaxPipelinesReconciler(testClient)

	result := reconcileAndGet(t, testClient, reconciler, pipeline.Name)
	require.NoError(t, result.err)

	assertCondition(t, result.pipeline,
		conditions.TypeConfigurationGenerated,
		metav1.ConditionFalse,
		conditions.ReasonMaxPipelinesExceeded,
		"Maximum pipeline count limit exceeded")

	assertCondition(t, result.pipeline,
		conditions.TypeFlowHealthy,
		metav1.ConditionFalse,
		conditions.ReasonSelfMonConfigNotGenerated,
		"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details")
}

func TestTLSConditions(t *testing.T) {
	tests := []struct {
		name                  string
		tlsCertErr            error
		expectedStatus        metav1.ConditionStatus
		expectedReason        string
		expectedMsg           string
		expectAgentConfigured bool
	}{
		{
			name:           "cert expired",
			tlsCertErr:     &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC)},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonTLSCertificateExpired,
			expectedMsg:    "TLS certificate expired on 2020-11-01",
		},
		{
			name:                  "cert about to expire",
			tlsCertErr:            &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC)},
			expectedStatus:        metav1.ConditionTrue,
			expectedReason:        conditions.ReasonTLSCertificateAboutToExpire,
			expectedMsg:           "TLS certificate is about to expire, configured certificate is valid until 2024-11-01",
			expectAgentConfigured: true,
		},
		{
			name:           "ca expired",
			tlsCertErr:     &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonTLSCertificateExpired,
			expectedMsg:    "TLS CA certificate expired on 2020-11-01",
		},
		{
			name:                  "ca about to expire",
			tlsCertErr:            &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
			expectedStatus:        metav1.ConditionTrue,
			expectedReason:        conditions.ReasonTLSCertificateAboutToExpire,
			expectedMsg:           "TLS CA certificate is about to expire, configured certificate is valid until 2024-11-01",
			expectAgentConfigured: true,
		},
		{
			name:           "cert decode failed",
			tlsCertErr:     tlscert.ErrCertDecodeFailed,
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonTLSConfigurationInvalid,
			expectedMsg:    "TLS configuration invalid: failed to decode PEM block containing certificate",
		},
		{
			name:           "key decode failed",
			tlsCertErr:     tlscert.ErrKeyDecodeFailed,
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonTLSConfigurationInvalid,
			expectedMsg:    "TLS configuration invalid: failed to decode PEM block containing private key",
		},
		{
			name:           "cert parse failed",
			tlsCertErr:     tlscert.ErrCertParseFailed,
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonTLSConfigurationInvalid,
			expectedMsg:    "TLS configuration invalid: failed to parse certificate",
		},
		{
			name:           "cert and key mismatch",
			tlsCertErr:     tlscert.ErrInvalidCertificateKeyPair,
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonTLSConfigurationInvalid,
			expectedMsg:    "TLS configuration invalid: certificate and private key do not match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewLogPipelineBuilder().
				WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
				WithHTTPOutput(testutils.HTTPClientTLSFromString("ca", "fooCert", "fooKey")).
				Build()
			testClient := newTestClient(t, &pipeline)
			reconciler := newTLSReconciler(testClient, tt.tlsCertErr)

			result := reconcileAndGet(t, testClient, reconciler, pipeline.Name)
			require.NoError(t, result.err)

			assertCondition(t, result.pipeline,
				conditions.TypeConfigurationGenerated,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMsg)

			// Check flow healthy condition when configuration generation fails
			if tt.expectedStatus == metav1.ConditionFalse {
				assertCondition(t, result.pipeline,
					conditions.TypeFlowHealthy,
					metav1.ConditionFalse,
					conditions.ReasonSelfMonConfigNotGenerated,
					"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details")
			}
		})
	}
}

func TestPodErrorConditions(t *testing.T) {
	tests := []struct {
		name           string
		probeErr       error
		expectedStatus metav1.ConditionStatus
		expectedReason string
		expectedMsg    string
	}{
		{
			name:           "pod is OOM",
			probeErr:       &workloadstatus.PodIsPendingError{ContainerName: "foo", Reason: "OOMKilled", Message: ""},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonAgentNotReady,
			expectedMsg:    "Pod is in the pending state because container: foo is not running due to: OOMKilled. Please check the container: foo logs.",
		},
		{
			name:           "pod is CrashLoop",
			probeErr:       &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonAgentNotReady,
			expectedMsg:    "Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
		},
		{
			name:           "no Pods deployed",
			probeErr:       workloadstatus.ErrNoPodsDeployed,
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonAgentNotReady,
			expectedMsg:    "No Pods deployed",
		},
		{
			name:           "fluent bit rollout in progress",
			probeErr:       &workloadstatus.RolloutInProgressError{},
			expectedStatus: metav1.ConditionTrue,
			expectedReason: conditions.ReasonRolloutInProgress,
			expectedMsg:    "Pods are being started/updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
			testClient := newTestClient(t, &pipeline)
			reconciler := newPodErrorReconciler(testClient, tt.probeErr)

			result := reconcileAndGet(t, testClient, reconciler, pipeline.Name)
			require.NoError(t, result.err)

			cond := meta.FindStatusCondition(result.pipeline.Status.Conditions, conditions.TypeAgentHealthy)
			require.Equal(t, tt.expectedStatus, cond.Status)
			require.Equal(t, tt.expectedReason, cond.Reason)
			require.Equal(t, tt.expectedMsg, cond.Message)
		})
	}
}

func TestUnsupportedMode(t *testing.T) {
	tests := []struct {
		name         string
		hasCustom    bool
		expectedMode bool
	}{
		{
			name:         "with custom plugin",
			hasCustom:    true,
			expectedMode: true,
		},
		{
			name:         "without custom plugin",
			hasCustom:    false,
			expectedMode: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipelineBuilder := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP")
			if tt.hasCustom {
				pipelineBuilder = pipelineBuilder.WithCustomFilter("Name grep")
			}

			pipeline := pipelineBuilder.Build()

			testClient := newTestClient(t, &pipeline)
			reconciler := newTestReconciler(testClient)

			result := reconcileAndGet(t, testClient, reconciler, pipeline.Name)
			require.NoError(t, result.err)
			require.Equal(t, tt.expectedMode, *result.pipeline.Status.UnsupportedMode)
		})
	}
}

func TestReferencedSecret(t *testing.T) {
	tests := []struct {
		name       string
		setupObjs  func() (telemetryv1alpha1.LogPipeline, []client.Object)
		secretErr  error
		expectErr  error
		conditions []conditionCheck
	}{
		{
			name: "API request failed",
			setupObjs: func() (telemetryv1alpha1.LogPipeline, []client.Object) {
				p := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
				return p, []client.Object{&p}
			},
			secretErr: &errortypes.APIRequestFailedError{Err: errors.New("server error")},
			expectErr: errors.New("server error"),
			conditions: []conditionCheck{
				{conditions.TypeConfigurationGenerated, metav1.ConditionFalse, conditions.ReasonValidationFailed, "Pipeline validation failed due to an error from the Kubernetes API server"},
				{conditions.TypeFlowHealthy, metav1.ConditionFalse, conditions.ReasonSelfMonConfigNotGenerated, "No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details"},
			},
		},
		{
			name: "secret missing",
			setupObjs: func() (telemetryv1alpha1.LogPipeline, []client.Object) {
				p := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
				return p, []client.Object{&p}
			},
			secretErr: fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound),
			conditions: []conditionCheck{
				{conditions.TypeConfigurationGenerated, metav1.ConditionFalse, conditions.ReasonReferencedSecretMissing, "One or more referenced Secrets are missing: Secret 'some-secret' of Namespace 'some-namespace'"},
				{conditions.TypeFlowHealthy, metav1.ConditionFalse, conditions.ReasonSelfMonConfigNotGenerated, "No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details"},
			},
		},
		{
			name: "referenced secret exists",
			setupObjs: func() (telemetryv1alpha1.LogPipeline, []client.Object) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-secret",
						Namespace: "some-namespace",
					},
					Data: map[string][]byte{"host": nil},
				}
				p := testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					WithHTTPOutput(testutils.HTTPHostFromSecret("some-secret", "some-namespace", "host")).
					Build()

				return p, []client.Object{&p, secret}
			},
			secretErr: nil,
			conditions: []conditionCheck{
				{conditions.TypeConfigurationGenerated, metav1.ConditionTrue, conditions.ReasonAgentConfigured, "LogPipeline specification is successfully applied to the configuration of Log agent"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline, objs := tt.setupObjs()
			testClient := newTestClientWithObjs(t, objs...)
			reconciler := newSecretRefReconciler(testClient, tt.secretErr)

			result := reconcileAndGet(t, testClient, reconciler, pipeline.Name)

			if tt.expectErr != nil {
				require.Error(t, result.err)
			} else {
				require.NoError(t, result.err)
			}

			for _, check := range tt.conditions {
				assertCondition(t, result.pipeline, check.condType, check.status, check.reason, check.message)
			}
		})
	}
}

func TestLogAgent(t *testing.T) {
	tests := []struct {
		name            string
		proberError     error
		errorConverter  any
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "log agent is not ready",
			proberError:     workloadstatus.ErrDaemonSetNotFound,
			errorConverter:  &commonStatusMocks.ErrorToMessageConverter{},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonAgentNotReady,
			expectedMessage: "DaemonSet is not yet created",
		},
		{
			name:            "log agent is ready",
			proberError:     nil,
			errorConverter:  &commonStatusMocks.ErrorToMessageConverter{},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  conditions.ReasonAgentReady,
			expectedMessage: "Log agent DaemonSet is ready",
		},
		{
			name:            "log agent prober fails",
			proberError:     workloadstatus.ErrDaemonSetFetching,
			errorConverter:  &conditions.ErrorToMessageConverter{},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonAgentNotReady,
			expectedMessage: "Failed to get DaemonSet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
			testClient := newTestClient(t, &pipeline)
			reconciler := newLogAgentReconciler(testClient, tt.proberError, tt.errorConverter)

			result := reconcileAndGet(t, testClient, reconciler, pipeline.Name)
			require.NoError(t, result.err)

			assertCondition(t, result.pipeline,
				conditions.TypeAgentHealthy,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMessage)
		})
	}
}

func TestFlowHealthy(t *testing.T) {
	tests := []struct {
		name           string
		probe          prober.FluentBitProbeResult
		probeErr       error
		expectedStatus metav1.ConditionStatus
		expectedReason string
		expectedMsg    string
	}{
		{
			name:           "prober fails",
			probeErr:       assert.AnError,
			expectedStatus: metav1.ConditionUnknown,
			expectedReason: conditions.ReasonSelfMonAgentProbingFailed,
			expectedMsg:    "Could not determine the health of the telemetry flow because the self monitor probing of agent failed",
		},
		{
			name: "healthy",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
			},
			expectedStatus: metav1.ConditionTrue,
			expectedReason: conditions.ReasonSelfMonFlowHealthy,
			expectedMsg:    "No problems detected in the telemetry flow",
		},
		{
			name: "buffer filling up",
			probe: prober.FluentBitProbeResult{
				BufferFillingUp: true,
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentBufferFillingUp,
			expectedMsg:    "Buffer nearing capacity. Incoming log rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=agent-buffer-filling-up",
		},
		{
			name: "no logs delivered",
			probe: prober.FluentBitProbeResult{
				NoLogsDelivered: true,
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentNoLogsDelivered,
			expectedMsg:    "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
		},
		{
			name: "no logs delivered shadows other problems",
			probe: prober.FluentBitProbeResult{
				NoLogsDelivered: true,
				BufferFillingUp: true,
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentNoLogsDelivered,
			expectedMsg:    "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
		},
		{
			name: "some data dropped",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentSomeDataDropped,
			expectedMsg:    "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=not-all-logs-arrive-at-the-backend",
		},
		{
			name: "some data dropped shadows other problems",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				BufferFillingUp:     true,
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentSomeDataDropped,
			expectedMsg:    "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=not-all-logs-arrive-at-the-backend",
		},
		{
			name: "all data dropped",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentAllDataDropped,
			expectedMsg:    "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
		},
		{
			name: "all data dropped shadows other problems",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{
					AllDataDropped:  true,
					SomeDataDropped: true,
				},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentAllDataDropped,
			expectedMsg:    "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
			testClient := newTestClient(t, &pipeline)
			reconciler := newFlowHealthReconciler(testClient, pipeline, tt.probe, tt.probeErr)

			result := reconcileAndGet(t, testClient, reconciler, pipeline.Name)
			require.NoError(t, result.err)

			cond := meta.FindStatusCondition(result.pipeline.Status.Conditions, conditions.TypeFlowHealthy)
			require.Equal(t, tt.expectedStatus, cond.Status)
			require.Equal(t, tt.expectedReason, cond.Reason)

			if tt.expectedMsg != "" {
				require.Equal(t, tt.expectedMsg, cond.Message)
			}
		})
	}
}

func TestFIPSMode(t *testing.T) {
	tests := []struct {
		name         string
		fipsEnabled  bool
		reconcilable bool
		conditions   []conditionCheck
	}{
		{
			name:         "FIPS mode enabled",
			fipsEnabled:  true,
			reconcilable: false,
			conditions: []conditionCheck{
				{conditions.TypeConfigurationGenerated, metav1.ConditionFalse, conditions.NoFluentbitInFipsMode, "FluentBit output is not available in FIPS mode"},
				{conditions.TypeFlowHealthy, metav1.ConditionFalse, conditions.ReasonSelfMonConfigNotGenerated, "No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details"},
			},
		},
		{
			name:         "FIPS mode disabled",
			fipsEnabled:  false,
			reconcilable: true,
			conditions: []conditionCheck{
				{conditions.TypeConfigurationGenerated, metav1.ConditionTrue, conditions.ReasonAgentConfigured, "LogPipeline specification is successfully applied to the configuration of Log agent"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
			testClient := newTestClient(t, &pipeline)
			reconciler := newFIPSReconciler(testClient, tt.fipsEnabled)

			// Test reconcilability
			reconcilable, err := reconciler.IsReconcilable(context.Background(), &pipeline)
			require.NoError(t, err)
			require.Equal(t, tt.reconcilable, reconcilable)

			// Test reconcile behavior
			result := reconcileAndGet(t, testClient, reconciler, pipeline.Name)
			require.NoError(t, result.err)

			for _, check := range tt.conditions {
				assertCondition(t, result.pipeline, check.condType, check.status, check.reason, check.message)
			}
		})
	}
}

func TestFIPSModeWithComplexPipelines(t *testing.T) {
	tests := []struct {
		name               string
		setupPipeline      func() telemetryv1alpha1.LogPipeline
		expectReconcilable bool
	}{
		{
			name: "FIPS mode ignores pipeline with TLS errors",
			setupPipeline: func() telemetryv1alpha1.LogPipeline {
				return testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					WithHTTPOutput(testutils.HTTPClientTLSFromString("ca", "invalidCert", "invalidKey")).
					Build()
			},
			expectReconcilable: false,
		},
		{
			name: "FIPS mode ignores pipeline with custom filters",
			setupPipeline: func() telemetryv1alpha1.LogPipeline {
				return testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					WithCustomFilter("Name grep").
					Build()
			},
			expectReconcilable: false,
		},
		{
			name: "FIPS mode ignores pipeline with secret references",
			setupPipeline: func() telemetryv1alpha1.LogPipeline {
				return testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					WithHTTPOutput(testutils.HTTPHostFromSecret("some-secret", "some-namespace", "host")).
					Build()
			},
			expectReconcilable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := tt.setupPipeline()
			testClient := newTestClient(t, &pipeline)
			reconciler := newFIPSReconciler(testClient, true) // Always FIPS enabled

			reconcilable, err := reconciler.IsReconcilable(context.Background(), &pipeline)
			require.NoError(t, err)
			require.Equal(t, tt.expectReconcilable, reconcilable)

			// In FIPS mode, all pipelines should get the same FIPS error condition regardless of other issues
			result := reconcileAndGet(t, testClient, reconciler, pipeline.Name)
			require.NoError(t, result.err)

			assertCondition(t, result.pipeline,
				conditions.TypeConfigurationGenerated,
				metav1.ConditionFalse,
				conditions.NoFluentbitInFipsMode,
				"FluentBit output is not available in FIPS mode")
		})
	}
}

// Helper types and functions
type reconcileResult struct {
	pipeline telemetryv1alpha1.LogPipeline
	err      error
}

type conditionCheck struct {
	condType string
	status   metav1.ConditionStatus
	reason   string
	message  string
}

func newTestClient(t *testing.T, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1alpha1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithStatusSubresource(objs...).Build()
}

func newTestClientWithObjs(t *testing.T, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1alpha1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithStatusSubresource(objs...).Build()
}

func reconcileAndGet(t *testing.T, client client.Client, reconciler *Reconciler, pipelineName string) reconcileResult {
	var pl telemetryv1alpha1.LogPipeline
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &pl))

	err := reconciler.Reconcile(context.Background(), &pl)

	var updatedPipeline telemetryv1alpha1.LogPipeline
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline))

	return reconcileResult{pipeline: updatedPipeline, err: err}
}

func assertCondition(t *testing.T, pipeline telemetryv1alpha1.LogPipeline, condType string, status metav1.ConditionStatus, reason, message string) {
	cond := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
	require.NotNil(t, cond)
	require.Equal(t, status, cond.Status)
	require.Equal(t, reason, cond.Reason)
	require.Equal(t, message, cond.Message)
}

func newTestReconciler(client client.Client) *Reconciler {
	return newReconcilerWithMocks(client, defaultMocks())
}

func newMaxPipelinesReconciler(client client.Client) *Reconciler {
	defaultMocks := defaultMocks()
	// Create a new pipeline lock with different behavior
	pipelineLock := &logpipelinefluentbitmocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)
	defaultMocks.pipelineLock = pipelineLock
	defaultMocks.validator.PipelineLock = pipelineLock

	return newReconcilerWithMocks(client, defaultMocks)
}

func newTLSReconciler(client client.Client, tlsErr error) *Reconciler {
	return newReconcilerWithOverrides(client, func(m *reconcilerMocks) {
		m.validator.TLSCertValidator = stubs.NewTLSCertValidator(tlsErr)
	})
}

func newPodErrorReconciler(client client.Client, probeErr error) *Reconciler {
	return newReconcilerWithOverrides(client, func(m *reconcilerMocks) {
		m.daemonSetProber = commonStatusStubs.NewDaemonSetProber(probeErr)
		m.errorConverter = &conditions.ErrorToMessageConverter{}
	})
}

func newSecretRefReconciler(client client.Client, secretErr error) *Reconciler {
	return newReconcilerWithOverrides(client, func(m *reconcilerMocks) {
		m.validator.SecretRefValidator = stubs.NewSecretRefValidator(secretErr)
	})
}

func newLogAgentReconciler(client client.Client, proberError error, errorConverter any) *Reconciler {
	return newReconcilerWithOverrides(client, func(m *reconcilerMocks) {
		m.daemonSetProber = commonStatusStubs.NewDaemonSetProber(proberError)

		switch v := errorConverter.(type) {
		case *commonStatusMocks.ErrorToMessageConverter:
			if proberError == nil {
				v.On("Convert", mock.Anything).Return("Log agent DaemonSet is ready")
			} else if errors.Is(proberError, workloadstatus.ErrDaemonSetNotFound) {
				v.On("Convert", mock.Anything).Return("DaemonSet is not yet created")
			}

			m.errorConverter = v
		case *conditions.ErrorToMessageConverter:
			m.errorConverter = v
		}
	})
}

func newFlowHealthReconciler(client client.Client, pipeline telemetryv1alpha1.LogPipeline, probe prober.FluentBitProbeResult, probeErr error) *Reconciler {
	defaultMocks := defaultMocks()
	// Override the flow health prober with specific behavior
	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(probe, probeErr)
	defaultMocks.flowHealthProber = flowHealthProber

	return newReconcilerWithMocks(client, defaultMocks)
}

func newFIPSReconciler(client client.Client, fipsEnabled bool) *Reconciler {
	return newReconcilerWithOverrides(client, func(m *reconcilerMocks) {
		// No specific overrides needed - just pass FIPS config through globals
	}, func(globals *config.Global) {
		*globals = config.NewGlobal(
			config.WithNamespace("default"),
			config.WithOperateInFIPSMode(fipsEnabled),
		)
	})
}

type reconcilerMocks struct {
	agentConfigBuilder  *logpipelinefluentbitmocks.AgentConfigBuilder
	agentApplierDeleter *logpipelinefluentbitmocks.AgentApplierDeleter
	daemonSetProber     AgentProber
	flowHealthProber    *logpipelinemocks.FlowHealthProber
	pipelineLock        *logpipelinefluentbitmocks.PipelineLock
	validator           *Validator
	errorConverter      ErrorToMessageConverter
}

func defaultMocks() reconcilerMocks {
	agentConfigBuilder := &logpipelinefluentbitmocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &logpipelinefluentbitmocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, mock.Anything).Return(prober.FluentBitProbeResult{}, nil)

	pipelineLock := &logpipelinefluentbitmocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	validator := &Validator{
		EndpointValidator:  stubs.NewEndpointValidator(nil),
		TLSCertValidator:   stubs.NewTLSCertValidator(nil),
		SecretRefValidator: stubs.NewSecretRefValidator(nil),
		PipelineLock:       pipelineLock,
	}

	errorConverter := &commonStatusMocks.ErrorToMessageConverter{}
	errorConverter.On("Convert", mock.Anything).Return("")

	return reconcilerMocks{
		agentConfigBuilder:  agentConfigBuilder,
		agentApplierDeleter: agentApplierDeleter,
		daemonSetProber:     commonStatusStubs.NewDaemonSetProber(nil),
		flowHealthProber:    flowHealthProber,
		pipelineLock:        pipelineLock,
		validator:           validator,
		errorConverter:      errorConverter,
	}
}

func newReconcilerWithMocks(client client.Client, mocks reconcilerMocks) *Reconciler {
	globals := config.NewGlobal(config.WithNamespace("default"))
	return newReconcilerWithGlobalConfig(client, mocks, globals)
}

func newReconcilerWithGlobalConfig(client client.Client, mocks reconcilerMocks, globals config.Global) *Reconciler {
	return New(
		globals,
		client,
		mocks.agentConfigBuilder,
		mocks.agentApplierDeleter,
		mocks.daemonSetProber,
		mocks.flowHealthProber,
		&stubs.IstioStatusChecker{IsActive: false},
		mocks.pipelineLock,
		mocks.validator,
		mocks.errorConverter,
	)
}

func newReconcilerWithOverrides(client client.Client, overrideFn func(*reconcilerMocks), configOverrides ...func(*config.Global)) *Reconciler {
	mocks := defaultMocks()
	overrideFn(&mocks)

	globals := config.NewGlobal(config.WithNamespace("default"))
	for _, override := range configOverrides {
		override(&globals)
	}

	return newReconcilerWithGlobalConfig(client, mocks, globals)
}

func containsPipelines(pp []telemetryv1alpha1.LogPipeline) any {
	return mock.MatchedBy(func(pipelines []telemetryv1alpha1.LogPipeline) bool {
		if len(pipelines) != len(pp) {
			return false
		}

		pipelineMap := make(map[string]bool)
		for _, p := range pipelines {
			pipelineMap[p.Name] = true
		}

		for _, p := range pp {
			if !pipelineMap[p.Name] {
				return false
			}
		}

		return true
	})
}
