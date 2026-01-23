package fluentbit

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
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

func TestAppInputDisabled(t *testing.T) {
	pipeline := testutils.NewLogPipelineBuilder().WithRuntimeInput(false).Build()
	testClient := newTestClient(t, &pipeline)
	reconciler := newTestReconciler(testClient)

	result := reconcileAndGet(t, testClient, reconciler, pipeline.Name)
	require.NoError(t, result.err)
}

func TestMaxPipelineLimit(t *testing.T) {
	pipeline := testutils.NewLogPipelineBuilder().
		WithCustomFilter("Name grep").
		Build()
	testClient := newTestClient(t, &pipeline)

	pipelineLock := &logpipelinefluentbitmocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)

	reconciler := newTestReconciler(testClient,
		WithPipelineLock(pipelineLock),
		WithPipelineValidator(newTestValidator(WithValidatorPipelineLock(pipelineLock))),
	)

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

func TestTLSCertificateValidation(t *testing.T) {
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
				WithHTTPOutput(testutils.HTTPClientTLSFromString("ca", "fooCert", "fooKey")).
				Build()
			testClient := newTestClient(t, &pipeline)

			reconciler := newTestReconciler(testClient,
				WithPipelineValidator(newTestValidator(WithTLSCertValidator(stubs.NewTLSCertValidator(tt.tlsCertErr)))),
			)

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

func TestPodErrorConditionReporting(t *testing.T) {
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
			pipeline := testutils.NewLogPipelineBuilder().Build()
			testClient := newTestClient(t, &pipeline)

			reconciler := newTestReconciler(testClient,
				WithAgentProber(commonStatusStubs.NewDaemonSetProber(tt.probeErr)),
			)

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
			pipelineBuilder := testutils.NewLogPipelineBuilder()
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

func TestSecretReferenceValidation(t *testing.T) {
	tests := []struct {
		name       string
		setupObjs  func() (telemetryv1beta1.LogPipeline, []client.Object)
		secretErr  error
		expectErr  error
		conditions []conditionCheck
	}{
		{
			name: "API request failed",
			setupObjs: func() (telemetryv1beta1.LogPipeline, []client.Object) {
				p := testutils.NewLogPipelineBuilder().Build()
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
			setupObjs: func() (telemetryv1beta1.LogPipeline, []client.Object) {
				p := testutils.NewLogPipelineBuilder().Build()
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
			setupObjs: func() (telemetryv1beta1.LogPipeline, []client.Object) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-secret",
						Namespace: "some-namespace",
					},
					Data: map[string][]byte{"host": nil},
				}
				p := testutils.NewLogPipelineBuilder().
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

			reconciler := newTestReconciler(testClient,
				WithPipelineValidator(newTestValidator(WithSecretRefValidator(stubs.NewSecretRefValidator(tt.secretErr)))),
			)

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

func TestAgentHealthCondition(t *testing.T) {
	tests := []struct {
		name            string
		proberError     error
		errorConverter  ErrorToMessageConverter
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "log agent is not ready",
			proberError:     workloadstatus.ErrDaemonSetNotFound,
			errorConverter:  &conditions.ErrorToMessageConverter{},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonAgentNotReady,
			expectedMessage: "DaemonSet is not yet created",
		},
		{
			name:            "log agent is ready",
			proberError:     nil,
			errorConverter:  &conditions.ErrorToMessageConverter{},
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
			pipeline := testutils.NewLogPipelineBuilder().Build()
			testClient := newTestClient(t, &pipeline)

			reconciler := newTestReconciler(testClient,
				WithAgentProber(commonStatusStubs.NewDaemonSetProber(tt.proberError)),
			)

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

func TestFlowHealthCondition(t *testing.T) {
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
			expectedMsg:    "Buffer nearing capacity. Incoming log rate exceeds export rate. See troubleshooting: " + conditions.LinkFluenBitBufferFillingUp,
		},
		{
			name: "no logs delivered",
			probe: prober.FluentBitProbeResult{
				NoLogsDelivered: true,
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentNoLogsDelivered,
			expectedMsg:    "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: " + conditions.LinkFluentBitNoLogsArriveAtBackend,
		},
		{
			name: "no logs delivered shadows other problems",
			probe: prober.FluentBitProbeResult{
				NoLogsDelivered: true,
				BufferFillingUp: true,
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentNoLogsDelivered,
			expectedMsg:    "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: " + conditions.LinkFluentBitNoLogsArriveAtBackend,
		},
		{
			name: "some data dropped",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentSomeDataDropped,
			expectedMsg:    "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: " + conditions.LinkFluentBitNotAllLogsArriveAtBackend,
		},
		{
			name: "some data dropped shadows other problems",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				BufferFillingUp:     true,
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentSomeDataDropped,
			expectedMsg:    "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: " + conditions.LinkFluentBitNotAllLogsArriveAtBackend,
		},
		{
			name: "all data dropped",
			probe: prober.FluentBitProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentAllDataDropped,
			expectedMsg:    "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: " + conditions.LinkFluentBitNoLogsArriveAtBackend,
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
			expectedMsg:    "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: " + conditions.LinkFluentBitNoLogsArriveAtBackend,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewLogPipelineBuilder().Build()
			testClient := newTestClient(t, &pipeline)

			flowHealthProber := &logpipelinemocks.FlowHealthProber{}
			flowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

			reconciler := newTestReconciler(testClient, WithFlowHealthProber(flowHealthProber))

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
		name               string
		fipsEnabled        bool
		pipeline           telemetryv1beta1.LogPipeline
		expectReconcilable bool
		verifyResources    bool
		conditions         []conditionCheck
	}{
		{
			name:               "FIPS disabled - basic pipeline applies resources",
			fipsEnabled:        false,
			pipeline:           testutils.NewLogPipelineBuilder().WithHTTPOutput().Build(),
			expectReconcilable: true,
			verifyResources:    true,
			conditions: []conditionCheck{
				{conditions.TypeConfigurationGenerated, metav1.ConditionTrue, conditions.ReasonAgentConfigured, "LogPipeline specification is successfully applied to the configuration of Log agent"},
			},
		},
		{
			name:               "FIPS disabled - HTTP output applies resources",
			fipsEnabled:        false,
			pipeline:           testutils.NewLogPipelineBuilder().WithHTTPOutput().Build(),
			expectReconcilable: true,
			verifyResources:    true,
			conditions: []conditionCheck{
				{conditions.TypeConfigurationGenerated, metav1.ConditionTrue, conditions.ReasonAgentConfigured, "LogPipeline specification is successfully applied to the configuration of Log agent"},
			},
		},
		{
			name:               "FIPS enabled - basic pipeline deletes resources",
			fipsEnabled:        true,
			pipeline:           testutils.NewLogPipelineBuilder().WithHTTPOutput().Build(),
			expectReconcilable: false,
			verifyResources:    true,
			conditions: []conditionCheck{
				{conditions.TypeConfigurationGenerated, metav1.ConditionFalse, conditions.ReasonNoFluentbitInFipsMode, "HTTP/custom output types are not supported when FIPS mode is enabled"},
				{conditions.TypeFlowHealthy, metav1.ConditionFalse, conditions.ReasonSelfMonConfigNotGenerated, "No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details"},
			},
		},
		{
			name:               "FIPS enabled - HTTP output deletes resources",
			fipsEnabled:        true,
			pipeline:           testutils.NewLogPipelineBuilder().WithHTTPOutput().Build(),
			expectReconcilable: false,
			verifyResources:    true,
			conditions: []conditionCheck{
				{conditions.TypeConfigurationGenerated, metav1.ConditionFalse, conditions.ReasonNoFluentbitInFipsMode, "HTTP/custom output types are not supported when FIPS mode is enabled"},
				{conditions.TypeFlowHealthy, metav1.ConditionFalse, conditions.ReasonSelfMonConfigNotGenerated, "No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details"},
			},
		},
		{
			name:               "FIPS enabled - pipeline with TLS errors",
			fipsEnabled:        true,
			pipeline:           testutils.NewLogPipelineBuilder().WithHTTPOutput(testutils.HTTPClientTLSFromString("ca", "invalidCert", "invalidKey")).Build(),
			expectReconcilable: false,
			verifyResources:    false,
			conditions: []conditionCheck{
				{conditions.TypeConfigurationGenerated, metav1.ConditionFalse, conditions.ReasonNoFluentbitInFipsMode, "HTTP/custom output types are not supported when FIPS mode is enabled"},
			},
		},
		{
			name:               "FIPS enabled - pipeline with custom filters",
			fipsEnabled:        true,
			pipeline:           testutils.NewLogPipelineBuilder().WithCustomFilter("Name grep").Build(),
			expectReconcilable: false,
			verifyResources:    false,
			conditions: []conditionCheck{
				{conditions.TypeConfigurationGenerated, metav1.ConditionFalse, conditions.ReasonNoFluentbitInFipsMode, "HTTP/custom output types are not supported when FIPS mode is enabled"},
			},
		},
		{
			name:               "FIPS enabled - pipeline with secret references",
			fipsEnabled:        true,
			pipeline:           testutils.NewLogPipelineBuilder().WithHTTPOutput(testutils.HTTPHostFromSecret("some-secret", "some-namespace", "host")).Build(),
			expectReconcilable: false,
			verifyResources:    false,
			conditions: []conditionCheck{
				{conditions.TypeConfigurationGenerated, metav1.ConditionFalse, conditions.ReasonNoFluentbitInFipsMode, "HTTP/custom output types are not supported when FIPS mode is enabled"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testClient := newTestClient(t, &tt.pipeline)

			reconcilerOpts := []Option{
				WithGlobals(config.NewGlobal(
					config.WithTargetNamespace("default"),
					config.WithOperateInFIPSMode(tt.fipsEnabled),
				)),
			}

			// Only set up resource verification when explicitly testing apply/delete behavior
			if tt.verifyResources {
				agentApplierDeleter := &logpipelinefluentbitmocks.AgentApplierDeleter{}
				if tt.fipsEnabled {
					agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
				} else {
					agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
				}

				reconcilerOpts = append(reconcilerOpts, WithAgentApplierDeleter(agentApplierDeleter))

				defer agentApplierDeleter.AssertExpectations(t)
			}

			reconciler := newTestReconciler(testClient, reconcilerOpts...)

			reconcilable, err := reconciler.IsReconcilable(t.Context(), &tt.pipeline)
			require.NoError(t, err)
			require.Equal(t, tt.expectReconcilable, reconcilable)

			result := reconcileAndGet(t, testClient, reconciler, tt.pipeline.Name)
			require.NoError(t, result.err)

			for _, check := range tt.conditions {
				assertCondition(t, result.pipeline, check.condType, check.status, check.reason, check.message)
			}
		})
	}
}

func TestPipelineInfoTracking(t *testing.T) {
	tests := []struct {
		name                 string
		pipeline             telemetryv1beta1.LogPipeline
		secret               *corev1.Secret
		expectedEndpoint     string
		expectedFeatureUsage []string
	}{
		{
			name: "basic HTTP output with runtime input",
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("pipeline-basic").
				WithHTTPOutput(testutils.HTTPHost("test")).
				WithRuntimeInput(true).
				Build(),
			expectedEndpoint: "test",
			expectedFeatureUsage: []string{
				metrics.FeatureInputRuntime,
				metrics.FeatureOutputHTTP,
			},
		},
		{
			name: "custom output",
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("pipeline-custom-output").
				WithCustomOutput("Name stdout").
				WithRuntimeInput(true).
				Build(),
			expectedEndpoint: "",
			expectedFeatureUsage: []string{
				metrics.FeatureOutputCustom,
				metrics.FeatureInputRuntime,
			},
		},
		{
			name: "custom filters",
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("pipeline-custom-filters").
				WithHTTPOutput(testutils.HTTPHost("test")).
				WithCustomFilter("Name grep").
				WithRuntimeInput(true).
				Build(),
			expectedEndpoint: "test",
			expectedFeatureUsage: []string{
				metrics.FeatureFilters,
				metrics.FeatureOutputHTTP,
				metrics.FeatureInputRuntime,
			},
		},
		{
			name: "variables",
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("pipeline-variables").
				WithHTTPOutput(testutils.HTTPHost("test")).
				WithVariable("var1", "secret1", "default", "key1").
				WithRuntimeInput(true).
				Build(),
			expectedEndpoint: "test",
			expectedFeatureUsage: []string{
				metrics.FeatureVariables,
				metrics.FeatureOutputHTTP,
				metrics.FeatureInputRuntime,
			},
		},
		{
			name: "files",
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("pipeline-files").
				WithHTTPOutput(testutils.HTTPHost("test")).
				WithFile("file1", "content1").
				WithRuntimeInput(true).
				Build(),
			expectedEndpoint: "test",
			expectedFeatureUsage: []string{
				metrics.FeatureFiles,
				metrics.FeatureOutputHTTP,
				metrics.FeatureInputRuntime,
			},
		},
		{
			name: "all FluentBit features",
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("pipeline-all-features").
				WithHTTPOutput(testutils.HTTPHost("test")).
				WithCustomFilter("Name grep").
				WithVariable("var1", "secret1", "default", "key1").
				WithFile("file1", "content1").
				WithRuntimeInput(true).
				Build(),
			expectedEndpoint: "test",
			expectedFeatureUsage: []string{
				metrics.FeatureOutputHTTP,
				metrics.FeatureFilters,
				metrics.FeatureVariables,
				metrics.FeatureFiles,
				metrics.FeatureInputRuntime,
			},
		},
		{
			name: "endpoint from secret",
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("pipeline-endpoint-secret").
				WithHTTPOutput(testutils.HTTPHostFromSecret("endpoint-secret", "default", "host")).
				Build(),
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "endpoint-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"host": []byte("endpoint.example.com"),
				},
			},
			expectedEndpoint: "endpoint.example.com",
			expectedFeatureUsage: []string{
				metrics.FeatureOutputHTTP,
			},
		},
		{
			name: "endpoint plain",
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("pipeline-endpoint-plain").
				WithHTTPOutput(testutils.HTTPHost("endpoint.example.com")).
				Build(),
			expectedEndpoint: "endpoint.example.com",
			expectedFeatureUsage: []string{
				metrics.FeatureOutputHTTP,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []client.Object{&tt.pipeline}
			if tt.secret != nil {
				objs = append(objs, tt.secret)
			}

			fakeClient := newTestClient(t, objs...)
			sut := newTestReconciler(fakeClient)

			result := reconcileAndGet(t, fakeClient, sut, tt.pipeline.Name)
			require.NoError(t, result.err)

			// Build expected label values
			labelValues := buildLabelValues(tt.pipeline.Name, tt.expectedEndpoint, tt.expectedFeatureUsage)

			metricValue := testutil.ToFloat64(metrics.LogPipelineInfo.WithLabelValues(labelValues...))
			require.Equal(t, float64(1), metricValue, "pipeline info metric should match for pipeline %q with endpoint %q and features %v", tt.pipeline.Name, tt.expectedEndpoint, tt.expectedFeatureUsage)
		})
	}
}

func buildLabelValues(pipelineName, endpoint string, enabledFeatures []string) []string {
	// Create a set of enabled features for quick lookup
	featuresSet := make(map[string]bool, len(enabledFeatures))
	for _, feature := range enabledFeatures {
		featuresSet[feature] = true
	}

	labelValues := []string{pipelineName, endpoint}

	for _, feature := range metrics.LogPipelineFeatures {
		if featuresSet[feature] {
			labelValues = append(labelValues, "true")
		} else {
			labelValues = append(labelValues, "false")
		}
	}

	return labelValues
}
