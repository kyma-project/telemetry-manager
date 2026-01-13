package tracepipeline

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
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/stubs"
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
			name:           "trace gateway probing failed",
			proberError:    workloadstatus.ErrDeploymentFetching,
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonGatewayNotReady,
			expectedMsg:    "Failed to get Deployment",
		},
		{
			name:           "trace gateway deployment is not ready",
			proberError:    &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: conditions.ReasonGatewayNotReady,
			expectedMsg:    "Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
		},
		{
			name:           "trace gateway deployment is ready",
			proberError:    nil,
			expectedStatus: metav1.ConditionTrue,
			expectedReason: conditions.ReasonGatewayReady,
			expectedMsg:    "Trace gateway Deployment is ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewTracePipelineBuilder().WithName("pipeline").Build()
			fakeClient := newTestClient(t, &pipeline)

			gatewayConfigBuilder := &mocks.GatewayConfigBuilder{}
			gatewayConfigBuilder.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Once()

			opts := []any{
				withGatewayConfigBuilderAssert(gatewayConfigBuilder),
			}
			if tt.proberError != nil {
				opts = append(opts, WithGatewayProber(commonStatusStubs.NewDeploymentSetProber(tt.proberError)))
			}

			sut, assertMocks := newTestReconciler(fakeClient, opts...)

			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeGatewayHealthy,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMsg)

			assertMocks(t)
		})
	}
}

func TestSecretReferenceValidation(t *testing.T) {
	tests := []struct {
		name                      string
		setupPipeline             func() telemetryv1beta1.TracePipeline
		setupSecret               func() *corev1.Secret
		includeSecret             bool
		secretValidatorError      error
		expectConfigGenerated     bool
		expectedConfigStatus      metav1.ConditionStatus
		expectedConfigReason      string
		expectedConfigMessage     string
		expectFlowHealthCondition bool
	}{
		{
			name: "referenced secret missing",
			setupPipeline: func() telemetryv1beta1.TracePipeline {
				return testutils.NewTracePipelineBuilder().
					WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).
					Build()
			},
			setupSecret:               func() *corev1.Secret { return nil },
			includeSecret:             false,
			secretValidatorError:      fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound),
			expectConfigGenerated:     false,
			expectedConfigStatus:      metav1.ConditionFalse,
			expectedConfigReason:      conditions.ReasonReferencedSecretMissing,
			expectedConfigMessage:     "One or more referenced Secrets are missing: Secret 'some-secret' of Namespace 'some-namespace'",
			expectFlowHealthCondition: true,
		},
		{
			name: "referenced secret exists",
			setupPipeline: func() telemetryv1beta1.TracePipeline {
				return testutils.NewTracePipelineBuilder().
					WithOTLPOutput(testutils.OTLPEndpointFromSecret("existing", "default", "endpoint")).
					Build()
			},
			setupSecret: func() *corev1.Secret {
				return &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing",
						Namespace: "default",
					},
					Data: map[string][]byte{"endpoint": nil},
				}
			},
			includeSecret:         true,
			secretValidatorError:  nil,
			expectConfigGenerated: true,
			expectedConfigStatus:  metav1.ConditionTrue,
			expectedConfigReason:  conditions.ReasonGatewayConfigured,
			expectedConfigMessage: "TracePipeline specification is successfully applied to the configuration of Trace gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := tt.setupPipeline()

			var clientObjs []client.Object

			clientObjs = append(clientObjs, &pipeline)
			if tt.includeSecret {
				clientObjs = append(clientObjs, tt.setupSecret())
			}

			fakeClient := newTestClient(t, clientObjs...)

			opts := []any{}

			if tt.secretValidatorError != nil {
				validator := newTestValidator(WithSecretRefValidator(stubs.NewSecretRefValidator(tt.secretValidatorError)))
				opts = append(opts, WithPipelineValidator(validator))
			}

			if tt.expectConfigGenerated {
				gatewayConfigBuilder := &mocks.GatewayConfigBuilder{}
				gatewayConfigBuilder.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Once()

				opts = append(opts, withGatewayConfigBuilderAssert(gatewayConfigBuilder))
			}

			sut, assertMocks := newTestReconciler(fakeClient, opts...)
			defer assertMocks(t)

			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeConfigurationGenerated,
				tt.expectedConfigStatus,
				tt.expectedConfigReason,
				tt.expectedConfigMessage)

			if tt.expectFlowHealthCondition {
				requireHasStatusCondition(t, result.pipeline,
					conditions.TypeFlowHealthy,
					metav1.ConditionFalse,
					conditions.ReasonSelfMonConfigNotGenerated,
					"No spans delivered to backend because TracePipeline specification is not applied to the configuration of Trace gateway. Check the 'ConfigurationGenerated' condition for more details")
			}
		})
	}
}

func TestMaxPipelineLimit(t *testing.T) {
	pipeline := testutils.NewTracePipelineBuilder().Build()
	fakeClient := newTestClient(t, &pipeline)

	pipelineLockStub := &mocks.PipelineLock{}
	pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)
	pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)

	gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
	// No On() setup - should not be called

	validator := newTestValidator(WithValidatorPipelineLock(pipelineLockStub))

	sut, assertMocks := newTestReconciler(fakeClient,
		WithPipelineLock(pipelineLockStub),
		WithPipelineValidator(validator),
		withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
	)
	defer assertMocks(t)

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
		"No spans delivered to backend because TracePipeline specification is not applied to the configuration of Trace gateway. Check the 'ConfigurationGenerated' condition for more details",
	)
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
			expectedMessage: "Trace gateway is unable to receive spans at current rate. See troubleshooting: " + conditions.LinkGatewayThrottling,
		},
		{
			name: "some data dropped",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewaySomeDataDropped,
			expectedMessage: "Backend is reachable, but rejecting spans. Some spans are dropped. See troubleshooting: " + conditions.LinkNotAllDataArriveAtBackend,
		},
		{
			name: "some data dropped shadows other problems",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				Throttling:          true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewaySomeDataDropped,
			expectedMessage: "Backend is reachable, but rejecting spans. Some spans are dropped. See troubleshooting: " + conditions.LinkNotAllDataArriveAtBackend,
		},
		{
			name: "all data dropped",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewayAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting spans. All spans are dropped. See troubleshooting: " + conditions.LinkNoDataArriveAtBackend,
		},
		{
			name: "all data dropped shadows other problems",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				Throttling:          true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewayAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting spans. All spans are dropped. See troubleshooting: " + conditions.LinkNoDataArriveAtBackend,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewTracePipelineBuilder().Build()
			fakeClient := newTestClient(t, &pipeline)

			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
			gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Times(1)

			flowHealthProberStub := &mocks.FlowHealthProber{}
			flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

			errToMsg := &conditions.ErrorToMessageConverter{}

			sut, assertMocks := newTestReconciler(
				fakeClient,
				WithFlowHealthProber(flowHealthProberStub),
				withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
				WithErrorToMessageConverter(errToMsg),
			)
			defer assertMocks(t)

			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeFlowHealthy,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMessage,
			)
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
			pipeline := testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPClientMTLSFromString("ca", "fooCert", "fooKey")).Build()
			fakeClient := newTestClient(t, &pipeline)

			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
			if tt.expectGatewayConfigured {
				gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Once()
			}
			// If not expectGatewayConfigured, leave mock without expectations -> will assert not called

			pipelineValidatorWithStubs := newTestValidator(
				WithSecretRefValidator(stubs.NewSecretRefValidator(tt.tlsCertErr)),
			)

			sut, assertMocks := newTestReconciler(
				fakeClient,
				WithPipelineValidator(pipelineValidatorWithStubs),
				withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
			)
			defer assertMocks(t)

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
					"No spans delivered to backend because TracePipeline specification is not applied to the configuration of Trace gateway. Check the 'ConfigurationGenerated' condition for more details",
				)
			}
		})
	}
}

func TestOTTLSpecValidation(t *testing.T) {
	tests := []struct {
		name             string
		validatorOption  func() ValidatorOption
		expectedErrorMsg string
	}{
		{
			name: "invalid transform spec",
			validatorOption: func() ValidatorOption {
				return WithTransformSpecValidator(stubs.NewTransformSpecValidator(
					&ottl.InvalidOTTLSpecError{Err: fmt.Errorf("invalid TransformSpec: error while parsing statements")},
				))
			},
			expectedErrorMsg: "OTTL specification is invalid, invalid TransformSpec: error while parsing statements. Fix the syntax error indicated by the message or see troubleshooting: " + conditions.LinkOTTLSpecInvalid,
		},
		{
			name: "invalid filter spec",
			validatorOption: func() ValidatorOption {
				return WithFilterSpecValidator(stubs.NewFilterSpecValidator(
					&ottl.InvalidOTTLSpecError{Err: fmt.Errorf("invalid FilterSpec: error while parsing conditions")},
				))
			},
			expectedErrorMsg: "OTTL specification is invalid, invalid FilterSpec: error while parsing conditions. Fix the syntax error indicated by the message or see troubleshooting: " + conditions.LinkOTTLSpecInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewTracePipelineBuilder().Build()
			fakeClient := newTestClient(t, &pipeline)

			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
			// No On() setup - should not be called

			validator := newTestValidator(tt.validatorOption())

			sut, assertMocks := newTestReconciler(
				fakeClient,
				withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
				WithPipelineValidator(validator),
			)
			defer assertMocks(t)

			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeConfigurationGenerated,
				metav1.ConditionFalse,
				conditions.ReasonOTTLSpecInvalid,
				tt.expectedErrorMsg,
			)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeFlowHealthy,
				metav1.ConditionFalse,
				conditions.ReasonSelfMonConfigNotGenerated,
				"No spans delivered to backend because TracePipeline specification is not applied to the configuration of Trace gateway. Check the 'ConfigurationGenerated' condition for more details",
			)
		})
	}
}

func TestAPIServerFailureHandling(t *testing.T) {
	serverErr := errors.New("failed to get lock: server error")

	tests := []struct {
		name            string
		setupPipeline   func() telemetryv1beta1.TracePipeline
		setupClient     func(*testing.T, *telemetryv1beta1.TracePipeline) client.Client
		setupReconciler func(client.Client, *mocks.GatewayConfigBuilder) (*testReconciler, func(*testing.T))
	}{
		{
			name: "a request to the Kubernetes API server has failed when validating the secret references",
			setupPipeline: func() telemetryv1beta1.TracePipeline {
				return testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPEndpointFromSecret(
					"existing",
					"default",
					"endpoint")).Build()
			},
			setupClient: func(t *testing.T, pipeline *telemetryv1beta1.TracePipeline) client.Client {
				secret := &corev1.Secret{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing",
						Namespace: "default",
					},
					Data: map[string][]byte{"endpoint": nil},
				}

				return newTestClient(t, pipeline, secret)
			},
			setupReconciler: func(fakeClient client.Client, gatewayConfigBuilderMock *mocks.GatewayConfigBuilder) (*testReconciler, func(*testing.T)) {
				validator := newTestValidator(
					WithSecretRefValidator(stubs.NewSecretRefValidator(&errortypes.APIRequestFailedError{Err: serverErr})),
				)

				return newTestReconciler(
					fakeClient,
					withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
					WithPipelineValidator(validator),
					WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),
				)
			},
		},
		{
			name: "a request to the Kubernetes API server has failed when validating the max pipeline count limit",
			setupPipeline: func() telemetryv1beta1.TracePipeline {
				return testutils.NewTracePipelineBuilder().WithName("pipeline").Build()
			},
			setupClient: func(t *testing.T, pipeline *telemetryv1beta1.TracePipeline) client.Client {
				return newTestClient(t, pipeline)
			},
			setupReconciler: func(fakeClient client.Client, gatewayConfigBuilderMock *mocks.GatewayConfigBuilder) (*testReconciler, func(*testing.T)) {
				pipelineLockStub := &mocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(&errortypes.APIRequestFailedError{Err: serverErr})

				validator := newTestValidator(WithValidatorPipelineLock(pipelineLockStub))

				return newTestReconciler(
					fakeClient,
					withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
					WithPipelineLock(pipelineLockStub),
					WithPipelineValidator(validator),
					WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := tt.setupPipeline()
			fakeClient := tt.setupClient(t, &pipeline)

			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
			// No On() setup - should not be called

			sut, assertMocks := tt.setupReconciler(fakeClient, gatewayConfigBuilderMock)
			defer assertMocks(t)

			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
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
				"No spans delivered to backend because TracePipeline specification is not applied to the configuration of Trace gateway. Check the 'ConfigurationGenerated' condition for more details",
			)
		})
	}
}

func TestNonReconcilablePipelines(t *testing.T) {
	pipeline := testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).Build()
	fakeClient := newTestClient(t, &pipeline)

	gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
	gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)

	pipelineValidatorWithStubs := newTestValidator(
		WithSecretRefValidator(stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound))),
	)

	errToMsg := &conditions.ErrorToMessageConverter{}

	sut, _ := newTestReconciler(
		fakeClient,
		WithGatewayApplierDeleter(gatewayApplierDeleterMock),
		WithPipelineValidator(pipelineValidatorWithStubs),
		WithErrorToMessageConverter(errToMsg),
	)
	result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
	require.NoError(t, result.err)

	// Manual assertion for this specific test
	gatewayApplierDeleterMock.AssertExpectations(t)
}

func TestPodErrorConditionReporting(t *testing.T) {
	tests := []struct {
		name            string
		probeGatewayErr error
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "pod is OOM",
			probeGatewayErr: &workloadstatus.PodIsPendingError{ContainerName: "foo", Reason: "OOMKilled"},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonGatewayNotReady,
			expectedMessage: "Pod is in the pending state because container: foo is not running due to: OOMKilled. Please check the container: foo logs.",
		},
		{
			name:            "pod is crashbackloop",
			probeGatewayErr: &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonGatewayNotReady,
			expectedMessage: "Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
		},
		{
			name:            "no Pods deployed",
			probeGatewayErr: workloadstatus.ErrNoPodsDeployed,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonGatewayNotReady,
			expectedMessage: "No Pods deployed",
		},
		{
			name:            "pod is ready",
			probeGatewayErr: nil,
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  conditions.ReasonGatewayReady,
			expectedMessage: conditions.MessageForTracePipeline(conditions.ReasonGatewayReady),
		},
		{
			name:            "rollout in progress",
			probeGatewayErr: &workloadstatus.RolloutInProgressError{},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  conditions.ReasonRolloutInProgress,
			expectedMessage: "Pods are being started/updated",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewTracePipelineBuilder().Build()
			fakeClient := newTestClient(t, &pipeline)

			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
			// TODO[k15r]: this mock was set up but never asserted
			// gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&common.Config{}, nil, nil).Once()

			pipelineValidatorWithStubs := newTestValidator(
				WithSecretRefValidator(stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound))),
			)

			gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(tt.probeGatewayErr)

			flowHealthProberStub := &mocks.FlowHealthProber{}
			flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil).Maybe()

			errToMsg := &conditions.ErrorToMessageConverter{}

			sut, assertMocks := newTestReconciler(
				fakeClient,
				withFlowHealthProberAssert(flowHealthProberStub),
				withGatewayConfigBuilderAssert(gatewayConfigBuilderMock),
				WithGatewayProber(gatewayProberStub),
				WithPipelineValidator(pipelineValidatorWithStubs),
				WithErrorToMessageConverter(errToMsg),
			)

			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			cond := meta.FindStatusCondition(result.pipeline.Status.Conditions, conditions.TypeGatewayHealthy)
			require.Equal(t, tt.expectedStatus, cond.Status)
			require.Equal(t, tt.expectedReason, cond.Reason)
			require.Equal(t, tt.expectedMessage, cond.Message)

			assertMocks(t)
		})
	}
}

func TestUsageTracking(t *testing.T) {
	tests := []struct {
		name                 string
		pipeline             telemetryv1beta1.TracePipeline
		expectedFeatureUsage map[string]float64
	}{
		{
			name:                 "pipeline without features",
			pipeline:             testutils.NewTracePipelineBuilder().WithName("pipeline-1").Build(),
			expectedFeatureUsage: map[string]float64{},
		},
		{
			name: "pipeline with transform",
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("pipeline-2").
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
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("pipeline-3").
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{"attributes[\"test\"] == \"value\""},
				}).
				Build(),
			expectedFeatureUsage: map[string]float64{
				metrics.FeatureFilter: 1,
			},
		},
		{
			name: "pipeline with transform and filter",
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("pipeline-4").
				WithTransform(telemetryv1beta1.TransformSpec{
					Statements: []string{"set(attributes[\"test\"], \"value\")"},
				}).
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{"attributes[\"test\"] == \"value\""},
				}).
				Build(),
			expectedFeatureUsage: map[string]float64{
				metrics.FeatureTransform: 1,
				metrics.FeatureFilter:    1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// All possible features that can be tracked
			allFeatures := []string{
				metrics.FeatureTransform,
				metrics.FeatureFilter,
			}

			oldFeatureUsage := map[string]float64{}
			for _, feature := range allFeatures {
				oldFeatureUsage[feature] = testutil.ToFloat64(metrics.TracePipelineFeatureUsage.WithLabelValues(feature, tt.pipeline.Name))
			}

			fakeClient := newTestClient(t, &tt.pipeline)

			sut, assertAll := newTestReconciler(fakeClient)

			result := reconcileAndGet(t, fakeClient, sut, tt.pipeline.Name)
			require.NoError(t, result.err)

			// Verify feature usage metrics for all features (default expected value is 0)
			for _, feature := range allFeatures {
				expectedValue := tt.expectedFeatureUsage[feature] // defaults to 0 if not in map
				newMetricValue := testutil.ToFloat64(metrics.TracePipelineFeatureUsage.WithLabelValues(feature, tt.pipeline.Name))
				oldMetricValue := oldFeatureUsage[feature]
				metricValue := newMetricValue - oldMetricValue
				require.Equal(t, expectedValue, metricValue, "feature usage metric should match for pipeline `%s` and feature `%s`", tt.pipeline.Name, feature)
			}

			assertAll(t)
		})
	}
}
