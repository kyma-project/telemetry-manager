package tracepipeline

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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/trace/gateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func TestReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	overridesHandlerStub := &mocks.OverridesHandler{}
	overridesHandlerStub.On("LoadOverrides", context.Background()).Return(&overrides.Config{}, nil)

	istioStatusCheckerStub := &mocks.IstioStatusChecker{}
	istioStatusCheckerStub.On("IsIstioActive", mock.Anything).Return(false)

	testConfig := Config{Gateway: otelcollector.GatewayConfig{
		Config: otelcollector.Config{
			BaseName:  "gateway",
			Namespace: "default",
		},
		Deployment: otelcollector.DeploymentConfig{
			Image: "otel/opentelemetry-collector-contrib",
		},
		OTLPServiceName: "otlp",
	}}

	t.Run("trace gateway probing failed", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().WithName("pipeline").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(workloadstatus.ErrDeploymentFetching)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		sut := New(
			fakeClient,
			testConfig,
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		err = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
		require.NoError(t, err)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionFalse,
			conditions.ReasonGatewayNotReady,
			workloadstatus.ErrDeploymentFetching.Error(),
		)

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("trace gateway deployment is not ready", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().WithName("pipeline").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(workloadstatus.ErrContainerCrashLoop)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		sut := New(
			fakeClient,
			testConfig,
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		err = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
		require.NoError(t, err)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionFalse,
			conditions.ReasonGatewayNotReady,
			workloadstatus.ErrContainerCrashLoop.Error(),
		)

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("trace gateway deployment is ready", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().WithName("pipeline").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		sut := New(
			fakeClient,
			testConfig,
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionTrue,
			conditions.ReasonGatewayReady,
			"Trace gateway Deployment is ready",
		)

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("referenced secret missing", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything).Return(&gateway.Config{}, nil, nil)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound)),
			PipelineLock:       pipelineLockStub,
		}

		sut := New(
			fakeClient,
			testConfig,
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			proberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonReferencedSecretMissing,
			"One or more referenced Secrets are missing: Secret 'some-secret' of Namespace 'some-namespace'",
		)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeFlowHealthy,
			metav1.ConditionFalse,
			conditions.ReasonSelfMonConfigNotGenerated,
			"No spans delivered to backend because TracePipeline specification is not applied to the configuration of Trace gateway. Check the 'ConfigurationGenerated' condition for more details",
		)

		gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
	})

	t.Run("referenced secret exists", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPEndpointFromSecret(
			"existing",
			"default",
			"endpoint")).Build()
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "existing",
				Namespace: "default",
			},
			Data: map[string][]byte{"endpoint": nil},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline, secret).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		sut := New(
			fakeClient,
			testConfig,
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionTrue,
			conditions.ReasonGatewayConfigured,
			"TracePipeline specification is successfully applied to the configuration of Trace gateway",
		)

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("max pipelines exceeded", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything).Return(&gateway.Config{}, nil, nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		sut := New(
			fakeClient,
			testConfig,
			flowHealthProberStub,
			&mocks.GatewayApplierDeleter{},
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.Error(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

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
			"No spans delivered to backend because TracePipeline specification is not applied to the configuration of Trace gateway. Check the 'ConfigurationGenerated' condition for more details",
		)

		gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
	})

	t.Run("flow healthy", func(t *testing.T) {
		tests := []struct {
			name            string
			probe           prober.OTelPipelineProbeResult
			probeErr        error
			expectedStatus  metav1.ConditionStatus
			expectedReason  string
			expectedMessage string
		}{
			{
				name:            "prober fails",
				probeErr:        assert.AnError,
				expectedStatus:  metav1.ConditionUnknown,
				expectedReason:  conditions.ReasonSelfMonProbingFailed,
				expectedMessage: "Could not determine the health of the telemetry flow because the self monitor probing failed",
			},
			{
				name: "healthy",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
				},
				expectedStatus:  metav1.ConditionTrue,
				expectedReason:  conditions.ReasonSelfMonFlowHealthy,
				expectedMessage: "No problems detected in the telemetry flow",
			},
			{
				name: "throttling",
				probe: prober.OTelPipelineProbeResult{
					Throttling: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonGatewayThrottling,
				expectedMessage: "Trace gateway is unable to receive spans at current rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-traces?id=gateway-throttling",
			},
			{
				name: "buffer filling up",
				probe: prober.OTelPipelineProbeResult{
					QueueAlmostFull: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
				expectedMessage: "Buffer nearing capacity. Incoming span rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-traces?id=gateway-buffer-filling-up",
			},
			{
				name: "buffer filling up shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					QueueAlmostFull: true,
					Throttling:      true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
				expectedMessage: "Buffer nearing capacity. Incoming span rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-traces?id=gateway-buffer-filling-up",
			},
			{
				name: "some data dropped",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting spans. Some spans are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-traces?id=not-all-spans-arrive-at-the-backend",
			},
			{
				name: "some data dropped shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
					Throttling:          true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting spans. Some spans are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-traces?id=not-all-spans-arrive-at-the-backend",
			},
			{
				name: "all data dropped",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting spans. All spans are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-traces?id=no-spans-arrive-at-the-backend",
			},
			{
				name: "all data dropped shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
					Throttling:          true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting spans. All spans are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-traces?id=no-spans-arrive-at-the-backend",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewTracePipelineBuilder().Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
				gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

				gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
				gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				pipelineLockStub := &mocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

				gatewayProberStub := &mocks.DeploymentProber{}
				gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

				flowHealthProberStub := &mocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

				pipelineValidatorWithStubs := &Validator{
					TLSCertValidator:   stubs.NewTLSCertValidator(nil),
					SecretRefValidator: stubs.NewSecretRefValidator(nil),
					PipelineLock:       pipelineLockStub,
				}

				sut := New(
					fakeClient,
					testConfig,
					flowHealthProberStub,
					gatewayApplierDeleterMock,
					gatewayConfigBuilderMock,
					gatewayProberStub,
					istioStatusCheckerStub,
					overridesHandlerStub,
					pipelineLockStub,
					pipelineValidatorWithStubs)
				_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.TracePipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				requireHasStatusCondition(t, updatedPipeline,
					conditions.TypeFlowHealthy,
					tt.expectedStatus,
					tt.expectedReason,
					tt.expectedMessage,
				)

				gatewayConfigBuilderMock.AssertExpectations(t)
			})
		}
	})

	t.Run("tls conditions", func(t *testing.T) {
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
				name:            "key parse failed",
				tlsCertErr:      tlscert.ErrKeyParseFailed,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSConfigurationInvalid,
				expectedMessage: "TLS configuration invalid: failed to parse private key",
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
				pipeline := testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPClientTLSFromString("ca", "fooCert", "fooKey")).Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
				gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything).Return(&gateway.Config{}, nil, nil)

				gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
				gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				pipelineLockStub := &mocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

				gatewayProberStub := &mocks.DeploymentProber{}
				gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

				flowHealthProberStub := &mocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

				pipelineValidatorWithStubs := &Validator{
					TLSCertValidator:   stubs.NewTLSCertValidator(nil),
					SecretRefValidator: stubs.NewSecretRefValidator(tt.tlsCertErr),
					PipelineLock:       pipelineLockStub,
				}

				sut := New(
					fakeClient,
					testConfig,
					flowHealthProberStub,
					gatewayApplierDeleterMock,
					gatewayConfigBuilderMock,
					gatewayProberStub,
					istioStatusCheckerStub,
					overridesHandlerStub,
					pipelineLockStub,
					pipelineValidatorWithStubs)
				_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.TracePipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

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
						"No spans delivered to backend because TracePipeline specification is not applied to the configuration of Trace gateway. Check the 'ConfigurationGenerated' condition for more details",
					)
				}

				if !tt.expectGatewayConfigured {
					gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
				} else {
					gatewayConfigBuilderMock.AssertCalled(t, "Build", mock.Anything, containsPipeline(pipeline))
				}
			})
		}
	})

	t.Run("a request to the Kubernetes API server has failed when validating the secret references", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPEndpointFromSecret(
			"existing",
			"default",
			"endpoint")).Build()
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "existing",
				Namespace: "default",
			},
			Data: map[string][]byte{"endpoint": nil},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline, secret).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything).Return(&gateway.Config{}, nil, nil)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		serverErr := errors.New("failed to get lock: server error")
		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(&errortypes.APIRequestFailedError{Err: serverErr}),
			PipelineLock:       pipelineLockStub,
		}

		sut := New(
			fakeClient,
			testConfig,
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.True(t, errors.Is(err, serverErr))

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonValidationFailed,
			"Pipeline validation failed due to an error from the Kubernetes API server",
		)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeFlowHealthy,
			metav1.ConditionFalse,
			conditions.ReasonSelfMonConfigNotGenerated,
			"No spans delivered to backend because TracePipeline specification is not applied to the configuration of Trace gateway. Check the 'ConfigurationGenerated' condition for more details",
		)

		gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
	})

	t.Run("a request to the Kubernetes API server has failed when validating the max pipeline count limit", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().WithName("pipeline").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything).Return(&gateway.Config{}, nil, nil)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		serverErr := errors.New("failed to get lock: server error")
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(&errortypes.APIRequestFailedError{Err: serverErr})

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		sut := New(
			fakeClient,
			testConfig,
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.True(t, errors.Is(err, serverErr))

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonValidationFailed,
			"Pipeline validation failed due to an error from the Kubernetes API server",
		)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeFlowHealthy,
			metav1.ConditionFalse,
			conditions.ReasonSelfMonConfigNotGenerated,
			"No spans delivered to backend because TracePipeline specification is not applied to the configuration of Trace gateway. Check the 'ConfigurationGenerated' condition for more details",
		)

		gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
	})

	t.Run("all trace pipelines are non-reconcilable", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything).Return(&gateway.Config{}, nil, nil)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound)),
			PipelineLock:       pipelineLockStub,
		}

		sut := New(
			fakeClient,
			testConfig,
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		gatewayApplierDeleterMock.AssertExpectations(t)
	})

	t.Run("Check different Pod Error Conditions", func(t *testing.T) {
		tests := []struct {
			name            string
			probeGatewayErr error
			expectedStatus  metav1.ConditionStatus
			expectedReason  string
			expectedMessage string
		}{
			{
				name:            "pod is OOM",
				probeGatewayErr: workloadstatus.ErrOOMKilled,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonGatewayNotReady,
				expectedMessage: workloadstatus.ErrOOMKilled.Error(),
			},
			{
				name:            "pod is craashbackloop",
				probeGatewayErr: workloadstatus.ErrContainerCrashLoop,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonGatewayNotReady,
				expectedMessage: workloadstatus.ErrContainerCrashLoop.Error(),
			},
			{
				name:            "no pods deployed",
				probeGatewayErr: workloadstatus.ErrNoPodsDeployed,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonGatewayNotReady,
				expectedMessage: workloadstatus.ErrNoPodsDeployed.Error(),
			},
			{
				name:            "pod is ready",
				probeGatewayErr: nil,
				expectedStatus:  metav1.ConditionTrue,
				expectedReason:  conditions.ReasonGatewayReady,
				expectedMessage: conditions.MessageForTracePipeline(conditions.ReasonGatewayReady),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewTracePipelineBuilder().Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
				gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

				gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
				gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				pipelineLockStub := &mocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

				gatewayProberStub := &mocks.DeploymentProber{}
				gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(tt.probeGatewayErr)

				flowHealthProberStub := &mocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

				sut := New(
					fakeClient,
					testConfig,
					flowHealthProberStub,
					gatewayApplierDeleterMock,
					gatewayConfigBuilderMock,
					gatewayProberStub,
					istioStatusCheckerStub,
					overridesHandlerStub,
					pipelineLockStub,
					stubs.NewTLSCertValidator(nil))

				_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.TracePipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeGatewayHealthy)
				require.Equal(t, tt.expectedStatus, cond.Status)
				require.Equal(t, tt.expectedReason, cond.Reason)
				require.Equal(t, tt.expectedMessage, cond.Message)
			})
		}
	})
}

func requireHasStatusCondition(t *testing.T, pipeline telemetryv1alpha1.TracePipeline, condType string, status metav1.ConditionStatus, reason, message string) {
	cond := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
	require.NotNil(t, cond, "could not find condition of type %s", condType)
	require.Equal(t, status, cond.Status)
	require.Equal(t, reason, cond.Reason)
	require.Equal(t, message, cond.Message)
	require.Equal(t, pipeline.Generation, cond.ObservedGeneration)
	require.NotEmpty(t, cond.LastTransitionTime)
}

func containsPipeline(p telemetryv1alpha1.TracePipeline) any {
	return mock.MatchedBy(func(pipelines []telemetryv1alpha1.TracePipeline) bool {
		return len(pipelines) == 1 && pipelines[0].Name == p.Name
	})
}
