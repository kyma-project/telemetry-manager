package metricpipeline

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func TestGatewayHealthCondition(t *testing.T) {
	tests := []struct {
		name           string
		proberError    error
		expectedStatus metav1.ConditionStatus
		expectedReason string
		expectedMsg    string
	}{
		{
			name:           "metric gateway deployment is not ready",
			proberError:    &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonGatewayNotReady,
			expectedMsg:    "Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
		},
		{
			name:           "metric gateway prober fails",
			proberError:    workloadstatus.ErrDeploymentFetching,
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonGatewayNotReady,
			expectedMsg:    "Failed to get Deployment",
		},
		{
			name:           "metric gateway deployment is ready",
			proberError:    nil,
			expectedStatus: metav1.ConditionTrue,
			expectedReason: conditions.ReasonGatewayReady,
			expectedMsg:    "Metric gateway Deployment is ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewMetricPipelineBuilder().Build()
			fakeClient := newTestClient(t, &pipeline)

			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
			gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Once()

			agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
			agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Once()

			reconcilerOpts := []any{
				withAgentApplierDeleterAssert(agentApplierDeleterMock),
				withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
			}

			if tt.proberError != nil {
				reconcilerOpts = append(reconcilerOpts, WithGatewayProber(commonStatusStubs.NewDeploymentSetProber(tt.proberError)))
			}

			sut, assertAll := newTestReconciler(fakeClient, reconcilerOpts...)

			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeGatewayHealthy,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMsg)

			assertAll(t)
		})
	}
}
func TestAgentHealthCondition(t *testing.T) {
	tests := []struct {
		name           string
		proberError    error
		expectedStatus metav1.ConditionStatus
		expectedReason string
		expectedMsg    string
	}{
		{
			name:           "metric agent daemonset is not ready",
			proberError:    &workloadstatus.PodIsPendingError{Message: "Error"},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonAgentNotReady,
			expectedMsg:    "Pod is in the pending state because container:  is not running due to: Error. Please check the container:  logs.",
		},
		{
			name:           "metric agent prober fails",
			proberError:    workloadstatus.ErrDaemonSetNotFound,
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonAgentNotReady,
			expectedMsg:    workloadstatus.ErrDaemonSetNotFound.Error(),
		},
		{
			name:           "metric agent daemonset is ready",
			proberError:    nil,
			expectedStatus: metav1.ConditionTrue,
			expectedReason: conditions.ReasonAgentReady,
			expectedMsg:    "Metric agent DaemonSet is ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
			fakeClient := newTestClient(t, &pipeline)

			agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
			agentConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Once()

			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
			gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Once()

			reconcilerOpts := []any{
				withAgentConfigBuilderAssert(agentConfigBuilderMock),
				withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
			}

			if tt.proberError != nil {
				reconcilerOpts = append(reconcilerOpts, WithAgentProber(commonStatusStubs.NewDaemonSetProber(tt.proberError)))
			}

			sut, assertAll := newTestReconciler(fakeClient, reconcilerOpts...)

			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeAgentHealthy,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMsg)

			assertAll(t)
		})
	}
}
func TestSecretReferenceValidation(t *testing.T) {
	t.Run("referenced secret exists", func(t *testing.T) {
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: "some-namespace",
			},
			Data: map[string][]byte{"user": {}, "password": {}},
		}
		pipeline := testutils.NewMetricPipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret(secret.Name, secret.Namespace, "user", "password")).Build()
		fakeClient := newTestClient(t, &pipeline)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Once()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Once()

		sut, assertAll := newTestReconciler(
			fakeClient,
			withAgentApplierDeleterAssert(agentApplierDeleterMock),
			withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
		)
		result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
		require.NoError(t, result.err)

		requireHasStatusCondition(t, result.pipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionTrue,
			conditions.ReasonGatewayConfigured,
			"MetricPipeline specification is successfully applied to the configuration of Metric gateway")

		assertAll(t)
	})

	t.Run("referenced secret missing", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).Build()
		fakeClient := newTestClient(t, &pipeline)

		customValidator := newTestValidator(
			WithSecretRefValidator(stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound))),
		)

		sut, assertAll := newTestReconciler(
			fakeClient,
			WithPipelineValidator(customValidator),
		)

		result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
		require.NoError(t, result.err)

		requireHasStatusCondition(t, result.pipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonReferencedSecretMissing,
			"One or more referenced Secrets are missing: Secret 'some-secret' of Namespace 'some-namespace'")

		requireHasStatusCondition(t, result.pipeline,
			conditions.TypeFlowHealthy,
			metav1.ConditionFalse,
			conditions.ReasonSelfMonConfigNotGenerated,
			"No metrics delivered to backend because MetricPipeline specification is not applied to the configuration of Metric gateway. Check the 'ConfigurationGenerated' condition for more details",
		)
		assertAll(t)
	})
}
func TestMaxPipelineLimit(t *testing.T) {
	pipeline := testutils.NewMetricPipelineBuilder().Build()
	fakeClient := newTestClient(t, &pipeline)

	agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
	agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Once()

	pipelineLockStub := &mocks.PipelineLock{}
	pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)
	pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)

	customValidator := newTestValidator(
		WithValidatorPipelineLock(pipelineLockStub),
	)

	sut, assertAll := newTestReconciler(
		fakeClient,
		withAgentApplierDeleterAssert(agentApplierDeleterMock),
		WithPipelineValidator(customValidator),
	)

	result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
	require.NoError(t, result.err)

	requireHasStatusCondition(t, result.pipeline,
		conditions.TypeConfigurationGenerated,
		metav1.ConditionFalse,
		conditions.ReasonMaxPipelinesExceeded,
		"Maximum pipeline count limit exceeded",
	)

	requireHasStatusCondition(t, result.pipeline,
		conditions.TypeFlowHealthy,
		metav1.ConditionFalse,
		conditions.ReasonSelfMonConfigNotGenerated,
		"No metrics delivered to backend because MetricPipeline specification is not applied to the configuration of Metric gateway. Check the 'ConfigurationGenerated' condition for more details",
	)
	assertAll(t)
}
func TestGatewayFlowHealthCondition(t *testing.T) {
	tests := []struct {
		name            string
		probe           prober.OTelGatewayProbeResult
		probeErr        error
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "prober fails",
			probeErr:        assert.AnError,
			expectedStatus:  metav1.ConditionUnknown,
			expectedReason:  conditions.ReasonSelfMonGatewayProbingFailed,
			expectedMessage: "Could not determine the health of the telemetry flow because the self monitor probing of gateway failed",
		},
		{
			name: "healthy",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
			},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  conditions.ReasonSelfMonFlowHealthy,
			expectedMessage: "No problems detected in the telemetry flow",
		},
		{
			name: "throttling",
			probe: prober.OTelGatewayProbeResult{
				Throttling: true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewayThrottling,
			expectedMessage: "Metric gateway is unable to receive metrics at current rate. See troubleshooting: " + conditions.LinkGatewayThrottling,
		},
		{
			name: "some data dropped",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewaySomeDataDropped,
			expectedMessage: "Backend is reachable, but rejecting metrics. Some metrics are dropped. See troubleshooting: " + conditions.LinkNotAllDataArriveAtBackend,
		},
		{
			name: "some data dropped shadows other problems",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				Throttling:          true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewaySomeDataDropped,
			expectedMessage: "Backend is reachable, but rejecting metrics. Some metrics are dropped. See troubleshooting: " + conditions.LinkNotAllDataArriveAtBackend,
		},
		{
			name: "all data dropped",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewayAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting metrics. All metrics are dropped. See troubleshooting: " + conditions.LinkNoDataArriveAtBackend,
		},
		{
			name: "all data dropped shadows other problems",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				Throttling:          true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewayAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting metrics. All metrics are dropped. See troubleshooting: " + conditions.LinkNoDataArriveAtBackend,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewMetricPipelineBuilder().Build()
			fakeClient := newTestClient(t, &pipeline)

			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
			gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Once()

			agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
			agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Once()

			gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
			gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			gatewayFlowHealthProberStub := &mocks.GatewayFlowHealthProber{}
			gatewayFlowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

			sut, assertAll := newTestReconciler(
				fakeClient,
				withAgentApplierDeleterAssert(agentApplierDeleterMock),
				withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
				WithGatewayFlowHealthProber(gatewayFlowHealthProberStub),
				WithGatewayApplierDeleter(gatewayApplierDeleterMock),
			)
			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeFlowHealthy,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMessage,
			)

			assertAll(t)
		})
	}
}
func TestAgentFlowHealthCondition(t *testing.T) {
	tests := []struct {
		name            string
		probe           prober.OTelAgentProbeResult
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
			probe: prober.OTelAgentProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
			},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  conditions.ReasonSelfMonFlowHealthy,
			expectedMessage: "No problems detected in the telemetry flow",
		},
		{
			name: "some data dropped",
			probe: prober.OTelAgentProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonSelfMonAgentSomeDataDropped,
			// TODO: Fix the documentation text in the link
			expectedMessage: "Backend is reachable, but rejecting metrics. Some metrics are dropped. See troubleshooting: " + conditions.LinkNotAllDataArriveAtBackend,
		},
		{
			name: "all data dropped",
			probe: prober.OTelAgentProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonAgentAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting metrics. All metrics are dropped. See troubleshooting: " + conditions.LinkNoDataArriveAtBackend,
		},
		{
			name: "all data dropped shadows other problems",
			probe: prober.OTelAgentProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true, SomeDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonAgentAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting metrics. All metrics are dropped. See troubleshooting: " + conditions.LinkNoDataArriveAtBackend,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
			fakeClient := newTestClient(t, &pipeline)

			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
			gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Once()

			agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
			agentConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Once()

			agentFlowHealthProberStub := &mocks.AgentFlowHealthProber{}
			agentFlowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

			sut, assertAll := newTestReconciler(
				fakeClient,
				withAgentConfigBuilderAssert(agentConfigBuilderMock),
				WithAgentFlowHealthProber(agentFlowHealthProberStub),
				withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
			)
			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeFlowHealthy,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMessage,
			)

			assertAll(t)
		})
	}
}
func TestTLSCertificateValidation(t *testing.T) {
	tests := []struct {
		name                    string
		tlsCertErr              error
		expectedStatus          metav1.ConditionStatus
		expectedReason          string
		expectedMessage         string
		expectGatewayConfigured bool
	}{
		{
			name:            "cert expired",
			tlsCertErr:      &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC)},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonTLSCertificateExpired,
			expectedMessage: "TLS certificate expired on 2020-11-01",
		},
		{
			name:                    "cert about to expire",
			tlsCertErr:              &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC)},
			expectedStatus:          metav1.ConditionTrue,
			expectedReason:          conditions.ReasonTLSCertificateAboutToExpire,
			expectedMessage:         "TLS certificate is about to expire, configured certificate is valid until 2024-11-01",
			expectGatewayConfigured: true,
		},
		{
			name:            "ca expired",
			tlsCertErr:      &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonTLSCertificateExpired,
			expectedMessage: "TLS CA certificate expired on 2020-11-01",
		},
		{
			name:                    "ca about to expire",
			tlsCertErr:              &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
			expectedStatus:          metav1.ConditionTrue,
			expectedReason:          conditions.ReasonTLSCertificateAboutToExpire,
			expectedMessage:         "TLS CA certificate is about to expire, configured certificate is valid until 2024-11-01",
			expectGatewayConfigured: true,
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
			pipeline := testutils.NewMetricPipelineBuilder().WithOTLPOutput(testutils.OTLPClientMTLSFromString("ca", "fooCert", "fooKey")).Build()
			fakeClient := newTestClient(t, &pipeline)

			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
			if tt.expectGatewayConfigured {
				gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Once()
			}

			agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
			agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil)

			gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
			gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			customValidator := newTestValidator(
				WithTLSCertValidator(stubs.NewTLSCertValidator(tt.tlsCertErr)),
			)

			sut, assertAll := newTestReconciler(
				fakeClient,
				WithAgentApplierDeleter(agentApplierDeleterMock),
				WithGatewayApplierDeleter(gatewayApplierDeleterMock),
				withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
				WithPipelineValidator(customValidator),
			)
			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeConfigurationGenerated,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMessage,
			)

			if tt.expectedStatus == metav1.ConditionFalse {
				requireHasStatusCondition(t, result.pipeline,
					conditions.TypeFlowHealthy,
					metav1.ConditionFalse,
					conditions.ReasonSelfMonConfigNotGenerated,
					"No metrics delivered to backend because MetricPipeline specification is not applied to the configuration of Metric gateway. Check the 'ConfigurationGenerated' condition for more details",
				)
			}

			assertAll(t)
		})
	}
}
func TestOTTLSpecValidation(t *testing.T) {
	tests := []struct {
		name            string
		validatorOption ValidatorOption
		expectedMessage string
	}{
		{
			name: "invalid transform spec",
			validatorOption: WithTransformSpecValidator(stubs.NewTransformSpecValidator(
				&ottl.InvalidOTTLSpecError{Err: fmt.Errorf("invalid TransformSpec: error while parsing statements")},
			)),
			expectedMessage: "OTTL specification is invalid, invalid TransformSpec: error while parsing statements. Fix the syntax error indicated by the message or see troubleshooting: " + conditions.LinkOTTLSpecInvalid,
		},
		{
			name: "invalid filter spec",
			validatorOption: WithFilterSpecValidator(stubs.NewFilterSpecValidator(
				&ottl.InvalidOTTLSpecError{Err: fmt.Errorf("invalid FilterSpec: error while parsing conditions")},
			)),
			expectedMessage: "OTTL specification is invalid, invalid FilterSpec: error while parsing conditions. Fix the syntax error indicated by the message or see troubleshooting: " + conditions.LinkOTTLSpecInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewMetricPipelineBuilder().Build()
			fakeClient := newTestClient(t, &pipeline)

			sut, assertAll := newTestReconciler(
				fakeClient,
				WithPipelineValidator(newTestValidator(tt.validatorOption)),
			)

			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeConfigurationGenerated,
				metav1.ConditionFalse,
				conditions.ReasonOTTLSpecInvalid,
				tt.expectedMessage,
			)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeFlowHealthy,
				metav1.ConditionFalse,
				conditions.ReasonSelfMonConfigNotGenerated,
				"No metrics delivered to backend because MetricPipeline specification is not applied to the configuration of Metric gateway. Check the 'ConfigurationGenerated' condition for more details",
			)
			assertAll(t)
		})
	}
}
func TestAPIServerFailureHandling(t *testing.T) {
	tests := []struct {
		name             string
		pipeline         telemetryv1beta1.MetricPipeline
		setupValidator   func(error) *Validator
		needsGatewayMock bool
	}{
		{
			name: "secret reference validation fails",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).
				Build(),
			setupValidator: func(serverErr error) *Validator {
				return newTestValidator(
					WithSecretRefValidator(stubs.NewSecretRefValidator(&errortypes.APIRequestFailedError{Err: serverErr})),
				)
			},
			needsGatewayMock: false,
		},
		{
			name:     "max pipeline count validation fails",
			pipeline: testutils.NewMetricPipelineBuilder().WithName("pipeline").Build(),
			setupValidator: func(serverErr error) *Validator {
				pipelineLock := &mocks.PipelineLock{}
				pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(&errortypes.APIRequestFailedError{Err: serverErr})

				return newTestValidator(WithValidatorPipelineLock(pipelineLock))
			},
			needsGatewayMock: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := newTestClient(t, &tt.pipeline)

			serverErr := errors.New("server error")

			agentMock := &mocks.AgentApplierDeleter{}
			// TODO[k15r]: in the original code this mock is set up with an expectation, but it is never called.
			// agentMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Once()

			opts := []any{
				withAgentApplierDeleterAssert(agentMock),
				WithPipelineValidator(tt.setupValidator(serverErr)),
			}

			if tt.needsGatewayMock {
				gatewayMock := &mocks.GatewayApplierDeleter{}
				gatewayMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				opts = append(opts, WithGatewayApplierDeleter(gatewayMock))
			}

			sut, assertAll := newTestReconciler(fakeClient, opts...)
			result := reconcileAndGet(t, fakeClient, sut, tt.pipeline.Name)
			require.ErrorIs(t, result.err, serverErr)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeConfigurationGenerated,
				metav1.ConditionFalse,
				conditions.ReasonValidationFailed,
				"Pipeline validation failed due to an error from the Kubernetes API server",
			)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeFlowHealthy,
				metav1.ConditionFalse,
				conditions.ReasonSelfMonConfigNotGenerated,
				"No metrics delivered to backend because MetricPipeline specification is not applied to the configuration of Metric gateway. Check the 'ConfigurationGenerated' condition for more details",
			)
			assertAll(t)
		})
	}
}
func TestNonReconcilablePipelines(t *testing.T) {
	pipeline := testutils.NewMetricPipelineBuilder().
		WithRuntimeInput(true).
		WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).
		Build()
	fakeClient := newTestClient(t, &pipeline)

	agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
	agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Once()

	gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
	gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	customValidator := newTestValidator(
		WithSecretRefValidator(stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound))),
	)

	sut, assertAll := newTestReconciler(
		fakeClient,
		withAgentApplierDeleterAssert(agentApplierDeleterMock),
		withGatewayApplierDeleterAssert(gatewayApplierDeleterMock),
		WithPipelineValidator(customValidator),
	)
	result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
	require.NoError(t, result.err)

	assertAll(t)
}

// TODO[k15r]: reduce complexity
func TestAgentRequirementDetermination(t *testing.T) { //nolint: gocognit // Complexity due to multiple test scenarios.
	tests := []struct {
		name                   string
		pipelineCount          int
		requireAgent           []bool
		expectedAgentDeletes   int
		expectedAgentApplies   int
		expectedGatewayApplies int
	}{
		{
			name:                   "one pipeline does not require agent",
			pipelineCount:          1,
			requireAgent:           []bool{false},
			expectedAgentDeletes:   1,
			expectedAgentApplies:   0,
			expectedGatewayApplies: 1,
		},
		{
			name:                   "some pipelines do not require agent",
			pipelineCount:          2,
			requireAgent:           []bool{false, true},
			expectedAgentDeletes:   0,
			expectedAgentApplies:   2,
			expectedGatewayApplies: 2,
		},
		{
			name:                   "all pipelines do not require agent",
			pipelineCount:          2,
			requireAgent:           []bool{false, false},
			expectedAgentDeletes:   2,
			expectedAgentApplies:   0,
			expectedGatewayApplies: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build pipelines and collect client objects
			var (
				clientObjs     []client.Object
				allPipelines   []telemetryv1beta1.MetricPipeline
				agentPipelines []telemetryv1beta1.MetricPipeline
			)

			for _, needsAgent := range tt.requireAgent {
				pipeline := testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(needsAgent).
					WithIstioInput(needsAgent).
					WithPrometheusInput(needsAgent).
					Build()
				allPipelines = append(allPipelines, pipeline)

				clientObjs = append(clientObjs, &allPipelines[len(allPipelines)-1])
				if needsAgent {
					agentPipelines = append(agentPipelines, pipeline)
				}
			}

			fakeClient := newTestClient(t, clientObjs...)

			// Setup mocks
			agentMock := &mocks.AgentApplierDeleter{}
			if tt.expectedAgentDeletes > 0 {
				agentMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Times(tt.expectedAgentDeletes)
			}

			if tt.expectedAgentApplies > 0 {
				agentMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(tt.expectedAgentApplies)
			}

			gatewayMock := &mocks.GatewayApplierDeleter{}
			if tt.expectedGatewayApplies > 0 {
				gatewayMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(tt.expectedGatewayApplies)
			}

			opts := []any{
				WithAgentApplierDeleter(agentMock),
				WithGatewayApplierDeleter(gatewayMock),
			}

			// Add config builders for multi-pipeline scenarios
			if tt.pipelineCount > 1 {
				gatewayConfigMock := &mocks.GatewayConfigBuilder{}
				gatewayConfigMock.On("Build", mock.Anything, containsPipelines(allPipelines), mock.Anything).Return(&common.Config{}, nil, nil)
				opts = append(opts, WithGatewayConfigBuilder(gatewayConfigMock))

				if len(agentPipelines) > 0 {
					agentConfigMock := &mocks.AgentConfigBuilder{}
					agentConfigMock.On("Build", mock.Anything, containsPipelines(agentPipelines), mock.Anything).Return(&common.Config{}, nil, nil)
					opts = append(opts, WithAgentConfigBuilder(agentConfigMock))
				}
			}

			sut, assertAll := newTestReconciler(fakeClient, opts...)

			// Reconcile and verify
			for i, pipeline := range allPipelines {
				result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
				require.NoError(t, result.err)

				if !tt.requireAgent[i] {
					requireHasStatusCondition(t, result.pipeline,
						conditions.TypeAgentHealthy,
						metav1.ConditionTrue,
						conditions.ReasonMetricAgentNotRequired,
						"")
				}
			}

			assertAll(t)
		})
	}
}
func TestPodErrorConditionReporting(t *testing.T) {
	tests := []struct {
		name            string
		probeAgentErr   error
		probeGatewayErr error
		conditionType   string
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "agent pod is OOM",
			probeAgentErr:   &workloadstatus.PodIsPendingError{ContainerName: "foo", Reason: "OOMKilled"},
			conditionType:   conditions.TypeAgentHealthy,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonAgentNotReady,
			expectedMessage: "Pod is in the pending state because container: foo is not running due to: OOMKilled. Please check the container: foo logs.",
		},
		{
			name:            "agent pod is CrashLoop",
			probeAgentErr:   &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
			conditionType:   conditions.TypeAgentHealthy,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonAgentNotReady,
			expectedMessage: "Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
		},
		{
			name:            "no agent pods deployed",
			probeAgentErr:   workloadstatus.ErrNoPodsDeployed,
			conditionType:   conditions.TypeAgentHealthy,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonAgentNotReady,
			expectedMessage: "No Pods deployed",
		},
		{
			name:            "gateway container not ready with error message",
			probeGatewayErr: &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
			conditionType:   conditions.TypeGatewayHealthy,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonGatewayNotReady,
			expectedMessage: "Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
		},
		{
			name:            "gateway container OOMKilled",
			probeGatewayErr: &workloadstatus.PodIsPendingError{ContainerName: "foo", Reason: "OOMKilled"},
			conditionType:   conditions.TypeGatewayHealthy,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonGatewayNotReady,
			expectedMessage: "Pod is in the pending state because container: foo is not running due to: OOMKilled. Please check the container: foo logs.",
		},
		{
			name:            "agent rollout in progress",
			probeAgentErr:   &workloadstatus.RolloutInProgressError{},
			conditionType:   conditions.TypeAgentHealthy,
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  conditions.ReasonRolloutInProgress,
			expectedMessage: "Pods are being started/updated",
		},
		{
			name:            "gateway rollout in progress",
			probeGatewayErr: &workloadstatus.RolloutInProgressError{},
			conditionType:   conditions.TypeGatewayHealthy,
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  conditions.ReasonRolloutInProgress,
			expectedMessage: "Pods are being started/updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
			fakeClient := newTestClient(t, &pipeline)

			agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
			agentConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Once()

			sut, assertAll := newTestReconciler(
				fakeClient,
				withAgentConfigBuilderAssert(agentConfigBuilderMock),
				WithAgentProber(commonStatusStubs.NewDaemonSetProber(tt.probeAgentErr)),
				WithGatewayProber(commonStatusStubs.NewDeploymentSetProber(tt.probeGatewayErr)),
			)

			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline, tt.conditionType, tt.expectedStatus, tt.expectedReason, tt.expectedMessage)
			assertAll(t)
		})
	}
}

func TestGetPipelinesRequiringAgents(t *testing.T) {
	r := Reconciler{}

	t.Run("no pipelines", func(t *testing.T) {
		pipelines := []telemetryv1beta1.MetricPipeline{}
		require.Empty(t, r.getPipelinesRequiringAgents(pipelines))
	})

	t.Run("no pipeline requires an agent", func(t *testing.T) {
		pipeline1 := testutils.NewMetricPipelineBuilder().WithRuntimeInput(false).WithPrometheusInput(false).WithIstioInput(false).Build()
		pipeline2 := testutils.NewMetricPipelineBuilder().WithRuntimeInput(false).WithPrometheusInput(false).WithIstioInput(false).Build()
		pipelines := []telemetryv1beta1.MetricPipeline{pipeline1, pipeline2}
		require.Empty(t, r.getPipelinesRequiringAgents(pipelines))
	})

	t.Run("some pipelines require an agent", func(t *testing.T) {
		pipeline1 := testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build()
		pipeline2 := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).WithIstioInput(true).Build()
		pipeline3 := testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).WithIstioInput(true).Build()
		pipeline4 := testutils.NewMetricPipelineBuilder().WithRuntimeInput(false).WithPrometheusInput(false).WithIstioInput(false).Build()
		pipelines := []telemetryv1beta1.MetricPipeline{pipeline1, pipeline2, pipeline3, pipeline4}
		require.ElementsMatch(t, []telemetryv1beta1.MetricPipeline{pipeline1, pipeline2, pipeline3}, r.getPipelinesRequiringAgents(pipelines))
	})

	t.Run("all pipelines require an agent", func(t *testing.T) {
		pipeline1 := testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build()
		pipeline2 := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
		pipeline3 := testutils.NewMetricPipelineBuilder().WithIstioInput(true).Build()
		pipelines := []telemetryv1beta1.MetricPipeline{pipeline1, pipeline2, pipeline3}
		require.ElementsMatch(t, []telemetryv1beta1.MetricPipeline{pipeline1, pipeline2, pipeline3}, r.getPipelinesRequiringAgents(pipelines))
	})
}

func TestUsageTracking(t *testing.T) {
	tests := []struct {
		name                 string
		pipeline             telemetryv1beta1.MetricPipeline
		expectedFeatureUsage map[string]float64
	}{
		{
			name: "pipeline without features",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("pipeline-1").
				WithOTLPInput(false).
				Build(),
			expectedFeatureUsage: map[string]float64{},
		},
		{
			name: "pipeline with transform",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("pipeline-2").
				WithOTLPInput(false).
				WithTransform(telemetryv1beta1.TransformSpec{
					Statements: []string{"set(attributes[\"test\"], \"value\")"},
				}).
				Build(),
			expectedFeatureUsage: map[string]float64{
				metrics.FeatureTransform: 1,
			},
		},
		{
			name: "pipeline with filter",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("pipeline-3").
				WithOTLPInput(false).
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{"resource.attributes[\"test\"] == \"value\""},
				}).
				Build(),
			expectedFeatureUsage: map[string]float64{
				metrics.FeatureFilter: 1,
			},
		},
		{
			name: "pipeline with transform and filter",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("pipeline-4").
				WithOTLPInput(false).
				WithTransform(telemetryv1beta1.TransformSpec{
					Statements: []string{"set(attributes[\"test\"], \"value\")"},
				}).
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{"resource.attributes[\"test\"] == \"value\""},
				}).
				Build(),
			expectedFeatureUsage: map[string]float64{
				metrics.FeatureTransform: 1,
				metrics.FeatureFilter:    1,
			},
		},
		{
			name: "pipeline with OTLP input",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("pipeline-5").
				WithOTLPInput(true).
				Build(),
			expectedFeatureUsage: map[string]float64{
				metrics.FeatureInputOTLP: 1,
			},
		},
		{
			name: "pipeline with runtime input",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("pipeline-6").
				WithOTLPInput(false).
				WithRuntimeInput(true).
				Build(),
			expectedFeatureUsage: map[string]float64{
				metrics.FeatureInputRuntime: 1,
			},
		},
		{
			name: "pipeline with prometheus input",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("pipeline-7").
				WithOTLPInput(false).
				WithPrometheusInput(true).
				Build(),
			expectedFeatureUsage: map[string]float64{
				metrics.FeatureInputPrometheus: 1,
			},
		},
		{
			name: "pipeline with istio input",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("pipeline-8").
				WithOTLPInput(false).
				WithIstioInput(true).
				Build(),
			expectedFeatureUsage: map[string]float64{
				metrics.FeatureInputIstio: 1,
			},
		},
		{
			name: "pipeline with all inputs",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("pipeline-9").
				WithRuntimeInput(true).
				WithPrometheusInput(true).
				WithIstioInput(true).
				WithOTLPInput(true).
				Build(),
			expectedFeatureUsage: map[string]float64{
				metrics.FeatureInputOTLP:       1,
				metrics.FeatureInputRuntime:    1,
				metrics.FeatureInputPrometheus: 1,
				metrics.FeatureInputIstio:      1,
			},
		},
		{
			name: "pipeline with all features",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("pipeline-10").
				WithTransform(telemetryv1beta1.TransformSpec{
					Statements: []string{"set(attributes[\"test\"], \"value\")"},
				}).
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{"resource.attributes[\"test\"] == \"value\""},
				}).
				WithRuntimeInput(true).
				WithPrometheusInput(true).
				WithIstioInput(true).
				WithOTLPInput(true).
				Build(),
			expectedFeatureUsage: map[string]float64{
				metrics.FeatureTransform:       1,
				metrics.FeatureFilter:          1,
				metrics.FeatureInputOTLP:       1,
				metrics.FeatureInputRuntime:    1,
				metrics.FeatureInputPrometheus: 1,
				metrics.FeatureInputIstio:      1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := newTestClient(t, &tt.pipeline)

			sut, assertAll := newTestReconciler(fakeClient)

			result := reconcileAndGet(t, fakeClient, sut, tt.pipeline.Name)
			require.NoError(t, result.err, "reconciliation should succeed but got error %v", result.err)

			// Verify feature usage metrics for all features (default expected value is 0)
			for _, feature := range metrics.AllFeatures {
				expectedValue := tt.expectedFeatureUsage[feature]
				metricValue := testutil.ToFloat64(metrics.MetricPipelineFeatureUsage.WithLabelValues(feature, tt.pipeline.Name))
				require.Equal(t, expectedValue, metricValue, "feature usage metric should match for pipeline `%s`and feature `%s`", tt.pipeline.Name, feature)
			}

			assertAll(t)
		})
	}
}
