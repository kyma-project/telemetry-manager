package metricpipeline

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
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/gateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	metricMocks "github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/stubs"
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

	overridesHandlerStub := &metricMocks.OverridesHandler{}
	overridesHandlerStub.On("LoadOverrides", context.Background()).Return(&overrides.Config{}, nil)

	istioStatusCheckerStub := &metricMocks.IstioStatusChecker{}
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

		gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &metricMocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &metricMocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(&workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"})

		flowHealthProberStub := &metricMocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			&metricMocks.AgentApplierDeleter{},
			&metricMocks.AgentConfigBuilder{},
			&metricMocks.DaemonSetProber{},
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsg,
		)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionFalse,
			conditions.ReasonGatewayNotReady,
			"Pod is in the pending state as container: foo is not running due to: Error")

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("metric gateway prober fails", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &metricMocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &metricMocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(workloadstatus.ErrDeploymentFetching)

		flowHealthProberStub := &metricMocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		errToMsg := &conditions.ErrorToMessageConverter{}

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		sut := New(
			fakeClient,
			testConfig,
			&metricMocks.AgentApplierDeleter{},
			&metricMocks.AgentConfigBuilder{},
			&metricMocks.DaemonSetProber{},
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsg,
		)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionFalse,
			conditions.ReasonGatewayNotReady,
			"Failed to get Deployment")

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("metric gateway deployment is ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &metricMocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &metricMocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &metricMocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			&metricMocks.AgentApplierDeleter{},
			&metricMocks.AgentConfigBuilder{},
			&metricMocks.DaemonSetProber{},
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsg,
		)
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

		agentConfigBuilderMock := &metricMocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&agent.Config{}).Times(1)

		gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		agentApplierDeleterMock := &metricMocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &metricMocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &metricMocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		agentProberStub := &metricMocks.DaemonSetProber{}
		agentProberStub.On("IsReady", mock.Anything, mock.Anything).Return(&workloadstatus.PodIsPendingError{Message: "Error"})

		flowHealthProberStub := &metricMocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}
		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			agentApplierDeleterMock,
			agentConfigBuilderMock,
			agentProberStub,
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsg,
		)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionFalse,
			conditions.ReasonAgentNotReady,
			"Pod is in the pending state as container:  is not running due to: Error")

		agentConfigBuilderMock.AssertExpectations(t)
		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("metric agent prober fails", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentConfigBuilderMock := &metricMocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&agent.Config{}).Times(1)

		gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		agentApplierDeleterMock := &metricMocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &metricMocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &metricMocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		agentProberMock := &metricMocks.DaemonSetProber{}
		agentProberMock.On("IsReady", mock.Anything, mock.Anything).Return(workloadstatus.ErrDaemonSetNotFound)

		flowHealthProberStub := &metricMocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			agentApplierDeleterMock,
			agentConfigBuilderMock,
			agentProberMock,
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsg,
		)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionFalse,
			conditions.ReasonAgentNotReady,
			workloadstatus.ErrDaemonSetNotFound.Error())

		agentConfigBuilderMock.AssertExpectations(t)
		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("metric agent daemonset is ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentConfigBuilderMock := &metricMocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&agent.Config{}).Times(1)

		gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		agentApplierDeleterMock := &metricMocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &metricMocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &metricMocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		agentProberMock := &metricMocks.DaemonSetProber{}
		agentProberMock.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &metricMocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			agentApplierDeleterMock,
			agentConfigBuilderMock,
			agentProberMock,
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsg,
		)
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

		gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &metricMocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &metricMocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &metricMocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			&metricMocks.AgentApplierDeleter{},
			&metricMocks.AgentConfigBuilder{},
			&metricMocks.DaemonSetProber{},
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsg,
		)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionTrue,
			conditions.ReasonGatewayConfigured,
			"MetricPipeline specification is successfully applied to the configuration of Metric gateway")

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("referenced secret missing", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil)

		agentApplierDeleterMock := &metricMocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil)

		gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &metricMocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &metricMocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &metricMocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound)),
			PipelineLock:       pipelineLockStub,
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			agentApplierDeleterMock,
			&metricMocks.AgentConfigBuilder{},
			&metricMocks.DaemonSetProber{},
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsg,
		)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonReferencedSecretMissing,
			"One or more referenced Secrets are missing: Secret 'some-secret' of Namespace 'some-namespace'")

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeFlowHealthy,
			metav1.ConditionFalse,
			conditions.ReasonSelfMonConfigNotGenerated,
			"No metrics delivered to backend because MetricPipeline specification is not applied to the configuration of Metric gateway. Check the 'ConfigurationGenerated' condition for more details",
		)

		gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
	})

	t.Run("max pipelines exceeded", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil)

		pipelineLockStub := &metricMocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)

		gatewayProberStub := &metricMocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &metricMocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}
		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			&metricMocks.AgentApplierDeleter{},
			&metricMocks.AgentConfigBuilder{},
			&metricMocks.DaemonSetProber{},
			flowHealthProberStub,
			&metricMocks.GatewayApplierDeleter{},
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsg,
		)
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

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeFlowHealthy,
			metav1.ConditionFalse,
			conditions.ReasonSelfMonConfigNotGenerated,
			"No metrics delivered to backend because MetricPipeline specification is not applied to the configuration of Metric gateway. Check the 'ConfigurationGenerated' condition for more details",
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
				expectedMessage: "Metric gateway is unable to receive metrics at current rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=gateway-throttling",
			},
			{
				name: "buffer filling up",
				probe: prober.OTelPipelineProbeResult{
					QueueAlmostFull: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
				expectedMessage: "Buffer nearing capacity. Incoming metric rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=gateway-buffer-filling-up",
			},
			{
				name: "buffer filling up shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					QueueAlmostFull: true,
					Throttling:      true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
				expectedMessage: "Buffer nearing capacity. Incoming metric rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=gateway-buffer-filling-up",
			},
			{
				name: "some data dropped",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting metrics. Some metrics are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=metrics-not-arriving-at-the-destination",
			},
			{
				name: "some data dropped shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
					Throttling:          true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting metrics. Some metrics are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=metrics-not-arriving-at-the-destination",
			},
			{
				name: "all data dropped",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting metrics. All metrics are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=no-metrics-arrive-at-the-backend",
			},
			{
				name: "all data dropped shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
					Throttling:          true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting metrics. All metrics are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=no-metrics-arrive-at-the-backend",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewMetricPipelineBuilder().Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
				gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

				gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
				gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				pipelineLockStub := &metricMocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

				gatewayProberStub := &metricMocks.DeploymentProber{}
				gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

				flowHealthProberStub := &metricMocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

				pipelineValidatorWithStubs := &Validator{
					TLSCertValidator:   stubs.NewTLSCertValidator(nil),
					SecretRefValidator: stubs.NewSecretRefValidator(nil),
					PipelineLock:       pipelineLockStub,
				}

				errToMsg := &conditions.ErrorToMessageConverter{}
				sut := New(
					fakeClient,
					testConfig,
					&metricMocks.AgentApplierDeleter{},
					&metricMocks.AgentConfigBuilder{},
					&metricMocks.DaemonSetProber{},
					flowHealthProberStub,
					gatewayApplierDeleterMock,
					gatewayConfigBuilderMock,
					gatewayProberStub,
					istioStatusCheckerStub,
					overridesHandlerStub,
					pipelineLockStub,
					pipelineValidatorWithStubs,
					errToMsg,
				)
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
				pipeline := testutils.NewMetricPipelineBuilder().WithOTLPOutput(testutils.OTLPClientTLSFromString("ca", "fooCert", "fooKey")).Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
				gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

				agentApplierDeleterMock := &metricMocks.AgentApplierDeleter{}
				agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil)

				gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
				gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				pipelineLockStub := &metricMocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

				gatewayProberStub := &metricMocks.DeploymentProber{}
				gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

				flowHealthProberStub := &metricMocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

				pipelineValidatorWithStubs := &Validator{
					TLSCertValidator:   stubs.NewTLSCertValidator(tt.tlsCertErr),
					SecretRefValidator: stubs.NewSecretRefValidator(nil),
					PipelineLock:       pipelineLockStub,
				}

				errToMsg := &conditions.ErrorToMessageConverter{}

				sut := New(
					fakeClient,
					testConfig,
					agentApplierDeleterMock,
					&metricMocks.AgentConfigBuilder{},
					&metricMocks.DaemonSetProber{},
					flowHealthProberStub,
					gatewayApplierDeleterMock,
					gatewayConfigBuilderMock,
					gatewayProberStub,
					istioStatusCheckerStub,
					overridesHandlerStub,
					pipelineLockStub,
					pipelineValidatorWithStubs,
					errToMsg,
				)
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

				if tt.expectedStatus == metav1.ConditionFalse {
					requireHasStatusCondition(t, updatedPipeline,
						conditions.TypeFlowHealthy,
						metav1.ConditionFalse,
						conditions.ReasonSelfMonConfigNotGenerated,
						"No metrics delivered to backend because MetricPipeline specification is not applied to the configuration of Metric gateway. Check the 'ConfigurationGenerated' condition for more details",
					)
				}

				if !tt.expectGatewayConfigured {
					gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
				} else {
					gatewayConfigBuilderMock.AssertExpectations(t)
				}
			})
		}

	})

	t.Run("a request to the Kubernetes API server has failed when validating the secret references", func(t *testing.T) {
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

		gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything).Return(&gateway.Config{}, nil, nil)

		gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &metricMocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &metricMocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &metricMocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		serverErr := errors.New("failed to get secret: server error")
		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(&errortypes.APIRequestFailedError{Err: serverErr}),
			PipelineLock:       pipelineLockStub,
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			&metricMocks.AgentApplierDeleter{},
			&metricMocks.AgentConfigBuilder{},
			&metricMocks.DaemonSetProber{},
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsg,
		)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.True(t, errors.Is(err, serverErr))

		var updatedPipeline telemetryv1alpha1.MetricPipeline
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
			"No metrics delivered to backend because MetricPipeline specification is not applied to the configuration of Metric gateway. Check the 'ConfigurationGenerated' condition for more details",
		)

		gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
	})

	t.Run("a request to the Kubernetes API server has failed when validating the max pipeline count limit", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithName("pipeline").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything).Return(&gateway.Config{}, nil, nil)

		gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		pipelineLockStub := &metricMocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		serverErr := errors.New("failed to get lock: server error")
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(&errortypes.APIRequestFailedError{Err: serverErr})

		gatewayProberStub := &metricMocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &metricMocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			&metricMocks.AgentApplierDeleter{},
			&metricMocks.AgentConfigBuilder{},
			&metricMocks.DaemonSetProber{},
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsg,
		)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.True(t, errors.Is(err, serverErr))

		var updatedPipeline telemetryv1alpha1.MetricPipeline
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
			"No metrics delivered to backend because MetricPipeline specification is not applied to the configuration of Metric gateway. Check the 'ConfigurationGenerated' condition for more details",
		)

		gatewayConfigBuilderMock.AssertNotCalled(t, "Build", mock.Anything, mock.Anything)
	})

	t.Run("all metric pipelines are non-reconcilable", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().
			WithRuntimeInput(true).
			WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).
			Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil)

		agentApplierDeleterMock := &metricMocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Times(1)

		gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)

		pipelineLockStub := &metricMocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := &metricMocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		agentProberStub := &metricMocks.DaemonSetProber{}
		agentProberStub.On("IsReady", mock.Anything, mock.Anything).Return(nil)

		flowHealthProberStub := &metricMocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound)),
			PipelineLock:       pipelineLockStub,
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			testConfig,
			agentApplierDeleterMock,
			&metricMocks.AgentConfigBuilder{},
			agentProberStub,
			flowHealthProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			overridesHandlerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsg,
		)
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		agentApplierDeleterMock.AssertExpectations(t)
		gatewayApplierDeleterMock.AssertExpectations(t)
	})

	t.Run("Check different Pod Error Conditions", func(t *testing.T) {
		tests := []struct {
			name            string
			probeAgentErr   error
			probeGatewayErr error
			expectedStatus  metav1.ConditionStatus
			expectedReason  string
			expectedMessage string
		}{
			{
				name:            "pod is OOM",
				probeAgentErr:   &workloadstatus.PodIsPendingError{ContainerName: "foo", Reason: "OOMKilled"},
				probeGatewayErr: nil,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonAgentNotReady,
				expectedMessage: "Pod is in the pending state as container: foo is not running due to: OOMKilled",
			},
			{
				name:            "pod is CrashLoop",
				probeAgentErr:   &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
				probeGatewayErr: nil,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonAgentNotReady,
				expectedMessage: "Pod is in the pending state as container: foo is not running due to: Error",
			},
			{
				name:            "no Pods deployed",
				probeAgentErr:   workloadstatus.ErrNoPodsDeployed,
				probeGatewayErr: nil,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonAgentNotReady,
				expectedMessage: "No Pods deployed",
			},
			{
				name:            "Container is not ready",
				probeAgentErr:   nil,
				probeGatewayErr: &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonGatewayNotReady,
				expectedMessage: "Pod is in the pending state as container: foo is not running due to: Error",
			},
			{
				name:            "Container is not ready",
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonGatewayNotReady,
				probeAgentErr:   nil,
				probeGatewayErr: &workloadstatus.PodIsPendingError{ContainerName: "foo", Reason: "OOMKilled"},
				expectedMessage: "Pod is in the pending state as container: foo is not running due to: OOMKilled",
			},
			{
				name:            "Agent Rollout in progress",
				expectedStatus:  metav1.ConditionTrue,
				expectedReason:  conditions.ReasonRolloutInProgress,
				probeAgentErr:   &workloadstatus.RolloutInProgressError{},
				probeGatewayErr: nil,
				expectedMessage: "Pods are being started/updated",
			},
			{
				name:            "Gateway Rollout in progress",
				expectedStatus:  metav1.ConditionTrue,
				expectedReason:  conditions.ReasonRolloutInProgress,
				probeAgentErr:   nil,
				probeGatewayErr: &workloadstatus.RolloutInProgressError{},
				expectedMessage: "Pods are being started/updated",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				agentConfigBuilderMock := &metricMocks.AgentConfigBuilder{}
				agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&agent.Config{}).Times(1)

				gatewayConfigBuilderMock := &metricMocks.GatewayConfigBuilder{}
				gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline)).Return(&gateway.Config{}, nil, nil).Times(1)

				agentApplierDeleterMock := &metricMocks.AgentApplierDeleter{}
				agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				gatewayApplierDeleterMock := &metricMocks.GatewayApplierDeleter{}
				gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				gatewayProberStub := &metricMocks.DeploymentProber{}
				agentProberMock := &metricMocks.DaemonSetProber{}

				agentProberMock.On("IsReady", mock.Anything, mock.Anything).Return(tt.probeAgentErr)
				gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(tt.probeGatewayErr)

				flowHealthProberStub := &metricMocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

				pipelineLockStub := &metricMocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

				pipelineValidatorWithStubs := &Validator{
					TLSCertValidator:   stubs.NewTLSCertValidator(nil),
					SecretRefValidator: stubs.NewSecretRefValidator(nil),
					PipelineLock:       pipelineLockStub,
				}

				errToMsg := &conditions.ErrorToMessageConverter{}

				sut := Reconciler{
					Client:                fakeClient,
					config:                testConfig,
					agentConfigBuilder:    agentConfigBuilderMock,
					gatewayConfigBuilder:  gatewayConfigBuilderMock,
					agentApplierDeleter:   agentApplierDeleterMock,
					gatewayApplierDeleter: gatewayApplierDeleterMock,
					pipelineLock:          pipelineLockStub,
					gatewayProber:         gatewayProberStub,
					agentProber:           agentProberMock,
					flowHealthProber:      flowHealthProberStub,
					overridesHandler:      overridesHandlerStub,
					istioStatusChecker:    istioStatusCheckerStub,
					pipelineValidator:     pipelineValidatorWithStubs,
					errToMsgConverter:     errToMsg,
				}
				_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.MetricPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				if tt.probeGatewayErr != nil {
					cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeGatewayHealthy)
					require.Equal(t, tt.expectedStatus, cond.Status)
					require.Equal(t, tt.expectedReason, cond.Reason)
					require.Equal(t, tt.expectedMessage, cond.Message)
				}
				if tt.probeAgentErr != nil {
					cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeAgentHealthy)
					require.Equal(t, tt.expectedStatus, cond.Status)
					require.Equal(t, tt.expectedReason, cond.Reason)
					require.Equal(t, tt.expectedMessage, cond.Message)
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
