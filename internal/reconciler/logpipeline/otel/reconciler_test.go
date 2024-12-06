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

	telemetryNamespace := "default"

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
			telemetryNamespace,
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
			telemetryNamespace,
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
			telemetryNamespace,
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
	// TODO: "referenced secret missing" (requires SecretRefValidator to be implemented)
	// TODO: "referenced secret exists" (requires SecretRefValidator to be implemented)
	// TODO: "flow healthy" (requires SelfMonitoring to be implemented)
	// TODO: "tls conditions" (requires TLSCertValidator to be implemented)
	// TODO: "all log pipelines are non-reconcilable" (requires SecretRefValidator to be implemented)
	// TODO: "Check different Pod Error Conditions" (requires SecretRefValidator to be implemented)
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
