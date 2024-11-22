package otel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log/gateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/otel/mocks"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func TestReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	overridesHandlerStub := &logpipelinemocks.OverridesHandler{}
	overridesHandlerStub.On("LoadOverrides", context.Background()).Return(&overrides.Config{}, nil)

	testConfig := Config{
		LogGatewayName:     "gateway",
		TelemetryNamespace: "default",
	}

	t.Run("log gateway probing failed", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(workloadstatus.ErrDeploymentFetching)

		// flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		// 		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			// EndpointValidator:  stubs.NewEndpointValidator(nil),
			// TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			// SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			pipelineValidatorWithStubs,
			errToMsg)
		err := sut.Reconcile(context.Background(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		err = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
		require.NoError(t, err)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionFalse,
			conditions.ReasonGatewayNotReady,
			"Failed to get Deployment",
		)

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("log gateway deployment is not ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(&workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"})

		// flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		// 		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			// 	EndpointValidator:  stubs.NewEndpointValidator(nil),
			// 	TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			// 	SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			pipelineValidatorWithStubs,
			errToMsg)
		err := sut.Reconcile(context.Background(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		err = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
		require.NoError(t, err)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionFalse,
			conditions.ReasonGatewayNotReady,
			"Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
		)

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("log gateway deployment is ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)

		// flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		// 		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			// EndpointValidator:  stubs.NewEndpointValidator(nil),
			// TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			// SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			pipelineValidatorWithStubs,
			errToMsg)
		err := sut.Reconcile(context.Background(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionTrue,
			conditions.ReasonGatewayReady,
			"Log gateway Deployment is ready",
		)

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	// TODO: Scenario requires SecretRefValidator to be implemented
	// t.Run("referenced secret missing", func(t *testing.T) {
	// 	pipeline := testutils.NewLogPipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).Build()
	// 	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

	// 	gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
	// 	gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything).Return(&gateway.Config{}, nil, nil)

	// 	gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
	// 	gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// 	gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)

	// 	// flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
	// 	// 		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

	// 	pipelineValidatorWithStubs := &Validator{
	// 		// EndpointValidator:  stubs.NewEndpointValidator(nil),
	// 		// TLSCertValidator:   stubs.NewTLSCertValidator(nil),
	// 		// SecretRefValidator: stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound)),
	// 	}

	// 	errToMsg := &conditions.ErrorToMessageConverter{}

	// 	sut := New(
	// 		fakeClient,
	// 		testConfig,
	// 		gatewayApplierDeleterMock,
	// 		gatewayConfigBuilderMock,
	// 		gatewayProberStub,
	// 		pipelineValidatorWithStubs,
	// 		errToMsg)
	// 	err := sut.Reconcile(context.Background(), &pipeline)
	// 	require.NoError(t, err)

	// 	var updatedPipeline telemetryv1alpha1.LogPipeline
	// 	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

	// 	requireHasStatusCondition(t, updatedPipeline,
	// 		conditions.TypeConfigurationGenerated,
	// 		metav1.ConditionFalse,
	// 		conditions.ReasonReferencedSecretMissing,
	// 		"One or more referenced Secrets are missing: Secret 'some-secret' of Namespace 'some-namespace'",
	// 	)

	// 	requireHasStatusCondition(t, updatedPipeline,
	// 		conditions.TypeFlowHealthy,
	// 		metav1.ConditionFalse,
	// 		conditions.ReasonSelfMonConfigNotGenerated,
	// 		"No spans delivered to backend because LogPipeline specification is not applied to the configuration of Log gateway. Check the 'ConfigurationGenerated' condition for more details",
	// 	)

	// 	gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
	// })

	// TODO: Scenario requires SecretRefValidator to be implemented
	// t.Run("referenced secret exists", func(t *testing.T) {
	// 	pipeline := testutils.NewLogPipelineBuilder().WithOTLPOutput(testutils.OTLPEndpointFromSecret(
	// 		"existing",
	// 		"default",
	// 		"endpoint")).Build()
	// 	secret := &corev1.Secret{
	// 		TypeMeta: metav1.TypeMeta{},
	// 		ObjectMeta: metav1.ObjectMeta{
	// 			Name:      "existing",
	// 			Namespace: "default",
	// 		},
	// 		Data: map[string][]byte{"endpoint": nil},
	// 	}
	// 	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline, secret).WithStatusSubresource(&pipeline).Build()

	// 	gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
	// 	gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

	// 	gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
	// 	gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// 	gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)

	// 	// flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
	// 	// 		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

	// 	pipelineValidatorWithStubs := &Validator{
	// 		// 	EndpointValidator:  stubs.NewEndpointValidator(nil),
	// 		// 	TLSCertValidator:   stubs.NewTLSCertValidator(nil),
	// 		// 	SecretRefValidator: stubs.NewSecretRefValidator(nil),
	// 	}

	// 	errToMsg := &conditions.ErrorToMessageConverter{}

	// 	sut := New(
	// 		fakeClient,
	// 		testConfig,
	// 		gatewayApplierDeleterMock,
	// 		gatewayConfigBuilderMock,
	// 		gatewayProberStub,
	// 		pipelineValidatorWithStubs,
	// 		errToMsg)
	// 	err := sut.Reconcile(context.Background(), &pipeline)
	// 	require.NoError(t, err)

	// 	var updatedPipeline telemetryv1alpha1.LogPipeline
	// 	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

	// 	requireHasStatusCondition(t, updatedPipeline,
	// 		conditions.TypeConfigurationGenerated,
	// 		metav1.ConditionTrue,
	// 		conditions.ReasonGatewayConfigured,
	// 		"LogPipeline specification is successfully applied to the configuration of Log gateway",
	// 	)

	// 	gatewayConfigBuilderMock.AssertExpectations(t)
	// })

	// TODO: Scenario requires SelfMonitoring to be implemented
	// t.Run("flow healthy", func(t *testing.T) {
	// 	tests := []struct {
	// 		name            string
	// 		probe           prober.OTelPipelineProbeResult
	// 		probeErr        error
	// 		expectedStatus  metav1.ConditionStatus
	// 		expectedReason  string
	// 		expectedMessage string
	// 	}{
	// 		{
	// 			name:            "prober fails",
	// 			probeErr:        assert.AnError,
	// 			expectedStatus:  metav1.ConditionUnknown,
	// 			expectedReason:  conditions.ReasonSelfMonProbingFailed,
	// 			expectedMessage: "Could not determine the health of the telemetry flow because the self monitor probing failed",
	// 		},
	// 		{
	// 			name: "healthy",
	// 			probe: prober.OTelPipelineProbeResult{
	// 				PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
	// 			},
	// 			expectedStatus:  metav1.ConditionTrue,
	// 			expectedReason:  conditions.ReasonSelfMonFlowHealthy,
	// 			expectedMessage: "No problems detected in the telemetry flow",
	// 		},
	// 		{
	// 			name: "throttling",
	// 			probe: prober.OTelPipelineProbeResult{
	// 				Throttling: true,
	// 			},
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonSelfMonGatewayThrottling,
	// 			expectedMessage: "Log gateway is unable to receive spans at current rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-logs?id=gateway-throttling",
	// 		},
	// 		{
	// 			name: "buffer filling up",
	// 			probe: prober.OTelPipelineProbeResult{
	// 				QueueAlmostFull: true,
	// 			},
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
	// 			expectedMessage: "Buffer nearing capacity. Incoming span rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-logs?id=gateway-buffer-filling-up",
	// 		},
	// 		{
	// 			name: "buffer filling up shadows other problems",
	// 			probe: prober.OTelPipelineProbeResult{
	// 				QueueAlmostFull: true,
	// 				Throttling:      true,
	// 			},
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
	// 			expectedMessage: "Buffer nearing capacity. Incoming span rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-logs?id=gateway-buffer-filling-up",
	// 		},
	// 		{
	// 			name: "some data dropped",
	// 			probe: prober.OTelPipelineProbeResult{
	// 				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
	// 			},
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
	// 			expectedMessage: "Backend is reachable, but rejecting spans. Some spans are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-logs?id=not-all-spans-arrive-at-the-backend",
	// 		},
	// 		{
	// 			name: "some data dropped shadows other problems",
	// 			probe: prober.OTelPipelineProbeResult{
	// 				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
	// 				Throttling:          true,
	// 			},
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
	// 			expectedMessage: "Backend is reachable, but rejecting spans. Some spans are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-logs?id=not-all-spans-arrive-at-the-backend",
	// 		},
	// 		{
	// 			name: "all data dropped",
	// 			probe: prober.OTelPipelineProbeResult{
	// 				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
	// 			},
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonSelfMonAllDataDropped,
	// 			expectedMessage: "Backend is not reachable or rejecting spans. All spans are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-logs?id=no-spans-arrive-at-the-backend",
	// 		},
	// 		{
	// 			name: "all data dropped shadows other problems",
	// 			probe: prober.OTelPipelineProbeResult{
	// 				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
	// 				Throttling:          true,
	// 			},
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonSelfMonAllDataDropped,
	// 			expectedMessage: "Backend is not reachable or rejecting spans. All spans are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-logs?id=no-spans-arrive-at-the-backend",
	// 		},
	// 	}

	// 	for _, tt := range tests {
	// 		t.Run(tt.name, func(t *testing.T) {
	// 			pipeline := testutils.NewLogPipelineBuilder().Build()
	// 			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

	// 			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
	// 			gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

	// 			gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
	// 			gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// 			gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)

	// 			flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
	// 			flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

	// 			pipelineValidatorWithStubs := &Validator{
	// 				// 	EndpointValidator:  stubs.NewEndpointValidator(nil),
	// 				// 	TLSCertValidator:   stubs.NewTLSCertValidator(nil),
	// 				// 	SecretRefValidator: stubs.NewSecretRefValidator(nil),
	// 			}

	// 			errToMsg := &conditions.ErrorToMessageConverter{}

	// 			sut := New(
	// 				fakeClient,
	// 				testConfig,
	// 				gatewayApplierDeleterMock,
	// 				gatewayConfigBuilderMock,
	// 				gatewayProberStub,
	// 				pipelineValidatorWithStubs,
	// 				errToMsg)
	// 			err := sut.Reconcile(context.Background(), &pipeline)
	// 			require.NoError(t, err)

	// 			var updatedPipeline telemetryv1alpha1.LogPipeline
	// 			_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

	// 			requireHasStatusCondition(t, updatedPipeline,
	// 				conditions.TypeFlowHealthy,
	// 				tt.expectedStatus,
	// 				tt.expectedReason,
	// 				tt.expectedMessage,
	// 			)

	// 			gatewayConfigBuilderMock.AssertExpectations(t)
	// 		})
	// 	}
	// })

	// TODO: Scenario requires TLSCertValidator to be implemented
	// t.Run("tls conditions", func(t *testing.T) {
	// 	tests := []struct {
	// 		name                    string
	// 		tlsCertErr              error
	// 		expectedStatus          metav1.ConditionStatus
	// 		expectedReason          string
	// 		expectedMessage         string
	// 		expectGatewayConfigured bool
	// 	}{
	// 		{
	// 			name:            "cert expired",
	// 			tlsCertErr:      &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC)},
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonTLSCertificateExpired,
	// 			expectedMessage: "TLS certificate expired on 2020-11-01",
	// 		},
	// 		{
	// 			name:                    "cert about to expire",
	// 			tlsCertErr:              &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC)},
	// 			expectedStatus:          metav1.ConditionTrue,
	// 			expectedReason:          conditions.ReasonTLSCertificateAboutToExpire,
	// 			expectedMessage:         "TLS certificate is about to expire, configured certificate is valid until 2024-11-01",
	// 			expectGatewayConfigured: true,
	// 		},
	// 		{
	// 			name:            "ca expired",
	// 			tlsCertErr:      &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonTLSCertificateExpired,
	// 			expectedMessage: "TLS CA certificate expired on 2020-11-01",
	// 		},
	// 		{
	// 			name:                    "ca about to expire",
	// 			tlsCertErr:              &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
	// 			expectedStatus:          metav1.ConditionTrue,
	// 			expectedReason:          conditions.ReasonTLSCertificateAboutToExpire,
	// 			expectedMessage:         "TLS CA certificate is about to expire, configured certificate is valid until 2024-11-01",
	// 			expectGatewayConfigured: true,
	// 		},
	// 		{
	// 			name:            "cert decode failed",
	// 			tlsCertErr:      tlscert.ErrCertDecodeFailed,
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonTLSConfigurationInvalid,
	// 			expectedMessage: "TLS configuration invalid: failed to decode PEM block containing certificate",
	// 		},
	// 		{
	// 			name:            "key decode failed",
	// 			tlsCertErr:      tlscert.ErrKeyDecodeFailed,
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonTLSConfigurationInvalid,
	// 			expectedMessage: "TLS configuration invalid: failed to decode PEM block containing private key",
	// 		},
	// 		{
	// 			name:            "key parse failed",
	// 			tlsCertErr:      tlscert.ErrKeyParseFailed,
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonTLSConfigurationInvalid,
	// 			expectedMessage: "TLS configuration invalid: failed to parse private key",
	// 		},
	// 		{
	// 			name:            "cert parse failed",
	// 			tlsCertErr:      tlscert.ErrCertParseFailed,
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonTLSConfigurationInvalid,
	// 			expectedMessage: "TLS configuration invalid: failed to parse certificate",
	// 		},
	// 		{
	// 			name:            "cert and key mismatch",
	// 			tlsCertErr:      tlscert.ErrInvalidCertificateKeyPair,
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonTLSConfigurationInvalid,
	// 			expectedMessage: "TLS configuration invalid: certificate and private key do not match",
	// 		},
	// 	}
	// 	for _, tt := range tests {
	// 		t.Run(tt.name, func(t *testing.T) {
	// 			pipeline := testutils.NewLogPipelineBuilder().WithOTLPOutput(testutils.OTLPClientTLSFromString("ca", "fooCert", "fooKey")).Build()
	// 			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

	// 			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
	// 			gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything).Return(&gateway.Config{}, nil, nil)

	// 			gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
	// 			gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// 			gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// 			gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)

	// 			flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
	// 			flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

	// 			pipelineValidatorWithStubs := &Validator{
	// 				// 	EndpointValidator:  stubs.NewEndpointValidator(nil),
	// 				// 	TLSCertValidator:   stubs.NewTLSCertValidator(nil),
	// 				// 	SecretRefValidator: stubs.NewSecretRefValidator(tt.tlsCertErr),
	// 			}

	// 			errToMsg := &conditions.ErrorToMessageConverter{}

	// 			sut := New(
	// 				fakeClient,
	// 				testConfig,
	// 				gatewayApplierDeleterMock,
	// 				gatewayConfigBuilderMock,
	// 				gatewayProberStub,
	// 				pipelineValidatorWithStubs,
	// 				errToMsg)
	// 			err := sut.Reconcile(context.Background(), &pipeline)
	// 			require.NoError(t, err)

	// 			var updatedPipeline telemetryv1alpha1.LogPipeline
	// 			_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

	// 			requireHasStatusCondition(t, updatedPipeline,
	// 				conditions.TypeConfigurationGenerated,
	// 				tt.expectedStatus,
	// 				tt.expectedReason,
	// 				tt.expectedMessage,
	// 			)

	// 			if tt.expectedStatus == metav1.ConditionFalse {
	// 				requireHasStatusCondition(t, updatedPipeline,
	// 					conditions.TypeFlowHealthy,
	// 					metav1.ConditionFalse,
	// 					conditions.ReasonSelfMonConfigNotGenerated,
	// 					"No spans delivered to backend because LogPipeline specification is not applied to the configuration of Log gateway. Check the 'ConfigurationGenerated' condition for more details",
	// 				)
	// 			}

	// 			if !tt.expectGatewayConfigured {
	// 				gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
	// 			} else {
	// 				gatewayConfigBuilderMock.AssertCalled(t, "Build", mock.Anything, containsPipeline(pipeline))
	// 			}
	// 		})
	// 	}
	// })

	// TODO: Scenario requires SecretRefValidator to be implemented
	// t.Run("all log pipelines are non-reconcilable", func(t *testing.T) {
	// 	pipeline := testutils.NewLogPipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).Build()
	// 	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

	// 	gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
	// 	gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything).Return(&gateway.Config{}, nil, nil)

	// 	gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
	// 	gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)

	// 	gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)

	// 	// flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
	// 	// 		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

	// 	pipelineValidatorWithStubs := &Validator{
	// 		// 	EndpointValidator:  stubs.NewEndpointValidator(nil),
	// 		// 	TLSCertValidator:   stubs.NewTLSCertValidator(nil),
	// 		// 	SecretRefValidator: stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound)),
	// 	}

	// 	errToMsg := &conditions.ErrorToMessageConverter{}

	// 	sut := New(
	// 		fakeClient,
	// 		testConfig,
	// 		gatewayApplierDeleterMock,
	// 		gatewayConfigBuilderMock,
	// 		gatewayProberStub,
	// 		pipelineValidatorWithStubs,
	// 		errToMsg)
	// 	err := sut.Reconcile(context.Background(), &pipeline)
	// 	require.NoError(t, err)

	// 	gatewayApplierDeleterMock.AssertExpectations(t)
	// })

	// TODO: Scenario requires SecretRefValidator to be implemented
	// t.Run("Check different Pod Error Conditions", func(t *testing.T) {
	// 	tests := []struct {
	// 		name            string
	// 		probeGatewayErr error
	// 		expectedStatus  metav1.ConditionStatus
	// 		expectedReason  string
	// 		expectedMessage string
	// 	}{
	// 		{
	// 			name:            "pod is OOM",
	// 			probeGatewayErr: &workloadstatus.PodIsPendingError{ContainerName: "foo", Reason: "OOMKilled"},
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonGatewayNotReady,
	// 			expectedMessage: "Pod is in the pending state because container: foo is not running due to: OOMKilled. Please check the container: foo logs.",
	// 		},
	// 		{
	// 			name:            "pod is crashbackloop",
	// 			probeGatewayErr: &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonGatewayNotReady,
	// 			expectedMessage: "Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
	// 		},
	// 		{
	// 			name:            "no Pods deployed",
	// 			probeGatewayErr: workloadstatus.ErrNoPodsDeployed,
	// 			expectedStatus:  metav1.ConditionFalse,
	// 			expectedReason:  conditions.ReasonGatewayNotReady,
	// 			expectedMessage: "No Pods deployed",
	// 		},
	// 		{
	// 			name:            "pod is ready",
	// 			probeGatewayErr: nil,
	// 			expectedStatus:  metav1.ConditionTrue,
	// 			expectedReason:  conditions.ReasonGatewayReady,
	// 			expectedMessage: conditions.MessageForLogPipeline(conditions.ReasonGatewayReady),
	// 		},
	// 		{
	// 			name:            "rollout in progress",
	// 			probeGatewayErr: &workloadstatus.RolloutInProgressError{},
	// 			expectedStatus:  metav1.ConditionTrue,
	// 			expectedReason:  conditions.ReasonRolloutInProgress,
	// 			expectedMessage: "Pods are being started/updated",
	// 		},
	// 	}
	// 	for _, tt := range tests {
	// 		t.Run(tt.name, func(t *testing.T) {
	// 			pipeline := testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()
	// 			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

	// 			gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
	// 			gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

	// 			gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
	// 			gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// 			pipelineValidatorWithStubs := &Validator{
	// 				// 	EndpointValidator:  stubs.NewEndpointValidator(nil),
	// 				// 	TLSCertValidator:   stubs.NewTLSCertValidator(nil),
	// 				// 	SecretRefValidator: stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound)),
	// 			}

	// 			gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(tt.probeGatewayErr)

	// 			flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
	// 			flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

	// 			errToMsg := &conditions.ErrorToMessageConverter{}

	// 			sut := New(
	// 				fakeClient,
	// 				testConfig,
	// 				gatewayApplierDeleterMock,
	// 				gatewayConfigBuilderMock,
	// 				gatewayProberStub,
	// 				pipelineValidatorWithStubs,
	// 				errToMsg)

	// 			err := sut.Reconcile(context.Background(), &pipeline)
	// 			require.NoError(t, err)

	// 			var updatedPipeline telemetryv1alpha1.LogPipeline
	// 			_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

	// 			cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeGatewayHealthy)
	// 			require.Equal(t, tt.expectedStatus, cond.Status)
	// 			require.Equal(t, tt.expectedReason, cond.Reason)
	// 			require.Equal(t, tt.expectedMessage, cond.Message)
	// 		})
	// 	}
	// })
}

func requireHasStatusCondition(t *testing.T, pipeline telemetryv1alpha1.LogPipeline, condType string, status metav1.ConditionStatus, reason, message string) {
	cond := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
	require.NotNil(t, cond, "could not find condition of type %s", condType)
	require.Equal(t, status, cond.Status)
	require.Equal(t, reason, cond.Reason)
	require.Equal(t, message, cond.Message)
	require.Equal(t, pipeline.Generation, cond.ObservedGeneration)
	require.NotEmpty(t, cond.LastTransitionTime)
}

func containsPipeline(p telemetryv1alpha1.LogPipeline) any {
	return mock.MatchedBy(func(pipelines []telemetryv1alpha1.LogPipeline) bool {
		return len(pipelines) == 1 && pipelines[0].Name == p.Name
	})
}
