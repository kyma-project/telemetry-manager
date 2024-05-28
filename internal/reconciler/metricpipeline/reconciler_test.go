package metricpipeline

import (
	"context"
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
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/gateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

func TestReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	overridesHandlerStub := &mocks.OverridesHandler{}
	overridesHandlerStub.On("LoadOverrides", context.Background()).Return(&overrides.Config{}, nil)

	istioStatusCheckerStub := &mocks.IstioStatusChecker{}
	istioStatusCheckerStub.On("IsIstioActive", mock.Anything).Return(false)

	testConfig := Config{
		Gateway: otelcollector.GatewayConfig{
			Config: otelcollector.Config{
				BaseName:  "gateway",
				Namespace: "default",
			},
			Deployment: otelcollector.DeploymentConfig{
				Image: "otel/opentelemetry-collector-contrib",
			},
			OTLPServiceName: "otlp",
		},
		Agent: otelcollector.AgentConfig{
			Config: otelcollector.Config{
				BaseName:  "agent",
				Namespace: "default",
			},
			DaemonSet: otelcollector.DaemonSetConfig{
				Image: "otel/opentelemetry-collector-contrib",
			},
		},
	}

	t.Run("metric gateway deployment is not ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client:               fakeClient,
			config:               testConfig,
			gatewayConfigBuilder: gatewayConfigBuilderMock,
			gatewayApplier:       &otelcollector.GatewayApplier{Config: testConfig.Gateway},
			pipelineLock:         pipelineLockStub,
			gatewayProber:        gatewayProberStub,
			overridesHandler:     overridesHandlerStub,
			istioStatusChecker:   istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionFalse,
			conditions.ReasonGatewayNotReady,
			"Metric gateway Deployment is not ready")

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("metric gateway prober fails", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, assert.AnError)

		sut := Reconciler{
			Client:               fakeClient,
			config:               testConfig,
			gatewayConfigBuilder: gatewayConfigBuilderMock,
			gatewayApplier:       &otelcollector.GatewayApplier{Config: testConfig.Gateway},
			pipelineLock:         pipelineLockStub,
			gatewayProber:        gatewayProberStub,
			overridesHandler:     overridesHandlerStub,
			istioStatusChecker:   istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionFalse,
			conditions.ReasonGatewayNotReady,
			"Metric gateway Deployment is not ready")

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("metric gateway deployment is ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:               fakeClient,
			config:               testConfig,
			gatewayConfigBuilder: gatewayConfigBuilderMock,
			gatewayApplier:       &otelcollector.GatewayApplier{Config: testConfig.Gateway},
			pipelineLock:         pipelineLockStub,
			gatewayProber:        gatewayProberStub,
			overridesHandler:     overridesHandlerStub,
			istioStatusChecker:   istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionTrue,
			conditions.ReasonGatewayReady,
			"Metric gateway Deployment is ready")

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("metric agent daemonset is not ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&agent.Config{}).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		agentProberStub := &mocks.DaemonSetProber{}
		agentProberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client:               fakeClient,
			config:               testConfig,
			agentConfigBuilder:   agentConfigBuilderMock,
			gatewayConfigBuilder: gatewayConfigBuilderMock,
			agentApplier:         &otelcollector.AgentApplier{Config: testConfig.Agent},
			gatewayApplier:       &otelcollector.GatewayApplier{Config: testConfig.Gateway},
			pipelineLock:         pipelineLockStub,
			gatewayProber:        gatewayProberStub,
			agentProber:          agentProberStub,
			overridesHandler:     overridesHandlerStub,
			istioStatusChecker:   istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionFalse,
			conditions.ReasonAgentNotReady,
			"Metric agent DaemonSet is not ready")

		agentConfigBuilderMock.AssertExpectations(t)
		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("metric agent prober fails", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&agent.Config{}).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		agentProberMock := &mocks.DaemonSetProber{}
		agentProberMock.On("IsReady", mock.Anything, mock.Anything).Return(false, assert.AnError)

		sut := Reconciler{
			Client:               fakeClient,
			config:               testConfig,
			agentConfigBuilder:   agentConfigBuilderMock,
			gatewayConfigBuilder: gatewayConfigBuilderMock,
			agentApplier:         &otelcollector.AgentApplier{Config: testConfig.Agent},
			gatewayApplier:       &otelcollector.GatewayApplier{Config: testConfig.Gateway},
			pipelineLock:         pipelineLockStub,
			gatewayProber:        gatewayProberStub,
			agentProber:          agentProberMock,
			overridesHandler:     overridesHandlerStub,
			istioStatusChecker:   istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionFalse,
			conditions.ReasonAgentNotReady,
			"Metric agent DaemonSet is not ready")

		agentConfigBuilderMock.AssertExpectations(t)
		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("metric agent daemonset is ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&agent.Config{}).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		agentProberMock := &mocks.DaemonSetProber{}
		agentProberMock.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:               fakeClient,
			config:               testConfig,
			agentConfigBuilder:   agentConfigBuilderMock,
			gatewayConfigBuilder: gatewayConfigBuilderMock,
			agentApplier:         &otelcollector.AgentApplier{Config: testConfig.Agent},
			gatewayApplier:       &otelcollector.GatewayApplier{Config: testConfig.Gateway},
			pipelineLock:         pipelineLockStub,
			gatewayProber:        gatewayProberStub,
			agentProber:          agentProberMock,
			overridesHandler:     overridesHandlerStub,
			istioStatusChecker:   istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionTrue,
			conditions.ReasonAgentReady,
			"Metric agent DaemonSet is ready")

		agentConfigBuilderMock.AssertExpectations(t)
		gatewayConfigBuilderMock.AssertExpectations(t)
	})

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
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline, secret).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:               fakeClient,
			config:               testConfig,
			gatewayConfigBuilder: gatewayConfigBuilderMock,
			gatewayApplier:       &otelcollector.GatewayApplier{Config: testConfig.Gateway},
			pipelineLock:         pipelineLockStub,
			gatewayProber:        gatewayProberStub,
			overridesHandler:     overridesHandlerStub,
			istioStatusChecker:   istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionTrue,
			conditions.ReasonAgentGatewayConfigured,
			"Metric agent and gateway successfully configured")

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("referenced secret missing", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:               fakeClient,
			config:               testConfig,
			gatewayConfigBuilder: gatewayConfigBuilderMock,
			pipelineLock:         pipelineLockStub,
			gatewayProber:        gatewayProberStub,
			overridesHandler:     overridesHandlerStub,
			istioStatusChecker:   istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonReferencedSecretMissing,
			"One or more referenced Secrets are missing")

		gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
	})

	t.Run("max pipelines exceeded", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(resourcelock.ErrLockInUse)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:               fakeClient,
			config:               testConfig,
			gatewayConfigBuilder: gatewayConfigBuilderMock,
			pipelineLock:         pipelineLockStub,
			gatewayProber:        gatewayProberStub,
			overridesHandler:     overridesHandlerStub,
			istioStatusChecker:   istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.Error(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonMaxPipelinesExceeded,
			"Maximum pipeline count limit exceeded",
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
				expectedMessage: "Metric gateway experiencing high influx: unable to receive metrics at current rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=gateway-throttling",
			},
			{
				name: "buffer filling up",
				probe: prober.OTelPipelineProbeResult{
					QueueAlmostFull: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
				expectedMessage: "Buffer nearing capacity: incoming metric rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=buffer-filling-up",
			},
			{
				name: "buffer filling up shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					QueueAlmostFull: true,
					Throttling:      true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
				expectedMessage: "Buffer nearing capacity: incoming metric rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=buffer-filling-up",
			},
			{
				name: "some data dropped",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Some metrics dropped: backend unreachable or rejecting. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=metrics-not-arriving-at-the-destination",
			},
			{
				name: "some data dropped shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
					Throttling:          true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Some metrics dropped: backend unreachable or rejecting. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=metrics-not-arriving-at-the-destination",
			},
			{
				name: "all data dropped",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "All metrics dropped: backend unreachable or rejecting. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=metrics-not-arriving-at-the-destination",
			},
			{
				name: "all data dropped shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
					Throttling:          true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "All metrics dropped: backend unreachable or rejecting. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=metrics-not-arriving-at-the-destination",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewMetricPipelineBuilder().Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
				gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

				pipelineLockStub := &mocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

				gatewayProberStub := &mocks.DeploymentProber{}
				gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

				flowHealthProberStub := &mocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

				sut := Reconciler{
					Client:               fakeClient,
					config:               testConfig,
					gatewayConfigBuilder: gatewayConfigBuilderMock,
					gatewayApplier:       &otelcollector.GatewayApplier{Config: testConfig.Gateway},
					pipelineLock:         pipelineLockStub,
					gatewayProber:        gatewayProberStub,
					flowHealthProber:     flowHealthProberStub,
					overridesHandler:     overridesHandlerStub,
					istioStatusChecker:   istioStatusCheckerStub,
				}
				_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.MetricPipeline
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
				name:            "cert decode failed",
				tlsCertErr:      tlscert.ErrCertDecodeFailed,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSCertificateInvalid,
				expectedMessage: "TLS certificate invalid: failed to decode PEM block containing cert",
			},
			{
				name:            "key decode failed",
				tlsCertErr:      tlscert.ErrKeyDecodeFailed,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSCertificateInvalid,
				expectedMessage: "TLS certificate invalid: failed to decode PEM block containing private key",
			},
			{
				name:            "key parse failed",
				tlsCertErr:      tlscert.ErrKeyParseFailed,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSCertificateInvalid,
				expectedMessage: "TLS certificate invalid: failed to parse private key",
			},
			{
				name:            "cert parse failed",
				tlsCertErr:      tlscert.ErrCertParseFailed,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSCertificateInvalid,
				expectedMessage: "TLS certificate invalid: failed to parse certificate",
			},
			{
				name:            "cert and key mismatch",
				tlsCertErr:      tlscert.ErrInvalidCertificateKeyPair,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSCertificateInvalid,
				expectedMessage: "TLS certificate invalid: certificate and private key do not match",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewMetricPipelineBuilder().WithOTLPOutput(testutils.OTLPClientTLS("ca", "fooCert", "fooKey")).Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
				gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

				pipelineLockStub := &mocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

				gatewayProberStub := &mocks.DeploymentProber{}
				gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)
				tlsStub := &mocks.TLSCertValidator{}
				tlsStub.On("ValidateCertificate", mock.Anything, mock.Anything, mock.Anything).Return(tt.tlsCertErr)

				sut := Reconciler{
					Client:               fakeClient,
					config:               testConfig,
					gatewayConfigBuilder: gatewayConfigBuilderMock,
					gatewayApplier:       &otelcollector.GatewayApplier{Config: testConfig.Gateway},
					pipelineLock:         pipelineLockStub,
					gatewayProber:        gatewayProberStub,
					tlsCertValidator:     tlsStub,
					overridesHandler:     overridesHandlerStub,
					istioStatusChecker:   istioStatusCheckerStub,
				}
				_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.MetricPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				requireHasStatusCondition(t, updatedPipeline,
					conditions.TypeConfigurationGenerated,
					tt.expectedStatus,
					tt.expectedReason,
					tt.expectedMessage,
				)

				if !tt.expectGatewayConfigured {
					gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
				} else {
					gatewayConfigBuilderMock.AssertExpectations(t)
				}
			})
		}

	})
}

func requireHasStatusCondition(t *testing.T, pipeline telemetryv1alpha1.MetricPipeline, condType string, status metav1.ConditionStatus, reason, message string) {
	cond := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
	require.NotNil(t, cond, "could not find condition of type %s", condType)
	require.Equal(t, status, cond.Status)
	require.Equal(t, reason, cond.Reason)
	require.Equal(t, message, cond.Message)
	require.Equal(t, pipeline.Generation, cond.ObservedGeneration)
	require.NotEmpty(t, cond.LastTransitionTime)
}

func containsPipeline(p telemetryv1alpha1.MetricPipeline) any {
	return mock.MatchedBy(func(pipelines []telemetryv1alpha1.MetricPipeline) bool {
		return len(pipelines) == 1 && pipelines[0].Name == p.Name
	})
}
