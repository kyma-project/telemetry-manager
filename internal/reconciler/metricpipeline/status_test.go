package metricpipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"k8s.io/apimachinery/pkg/api/meta"
)

func TestUpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("metric gateway deployment is not ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayProberMock := &mocks.DeploymentProber{}
		fakeGatewayName := types.NamespacedName{Name: "gateway", Namespace: "telemetry"}
		gatewayProberMock.On("IsReady", mock.Anything, fakeGatewayName).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: otelcollector.GatewayConfig{
				Config: otelcollector.Config{
					BaseName:  fakeGatewayName.Name,
					Namespace: fakeGatewayName.Namespace,
				},
			}},
			gatewayProber: gatewayProberMock,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeMetricGatewayReady)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeMetricGatewayReady)
		require.Equal(t, metav1.ConditionFalse, cond.Status)
		require.Equal(t, conditions.ReasonMetricGatewayDeploymentNotReady, cond.Reason)

		mock.AssertExpectationsForObjects(t, gatewayProberMock)
	})

	t.Run("metric gateway deployment is ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayProberMock := &mocks.DeploymentProber{}
		fakeGatewayName := types.NamespacedName{Name: "gateway", Namespace: "telemetry"}
		gatewayProberMock.On("IsReady", mock.Anything, fakeGatewayName).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: otelcollector.GatewayConfig{
				Config: otelcollector.Config{
					BaseName:  fakeGatewayName.Name,
					Namespace: fakeGatewayName.Namespace,
				},
			}},
			gatewayProber: gatewayProberMock,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeMetricGatewayReady)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeMetricGatewayReady)
		require.Equal(t, metav1.ConditionTrue, cond.Status)
		require.Equal(t, conditions.ReasonMetricGatewayDeploymentReady, cond.Reason)

		mock.AssertExpectationsForObjects(t, gatewayProberMock)
	})

	t.Run("metric agent daemonset is not ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().PrometheusInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		agentProberMock := &mocks.DaemonSetProber{}
		fakeAgentName := types.NamespacedName{Name: "agent", Namespace: "telemetry"}
		agentProberMock.On("IsReady", mock.Anything, fakeAgentName).Return(false, nil)
		sut := Reconciler{
			Client: fakeClient,
			config: Config{Agent: otelcollector.AgentConfig{
				Config: otelcollector.Config{
					BaseName:  fakeAgentName.Name,
					Namespace: fakeAgentName.Namespace,
				},
			}},
			gatewayProber: gatewayProberStub,
			agentProber:   agentProberMock,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeMetricAgentReady)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeMetricAgentReady)
		require.Equal(t, metav1.ConditionFalse, cond.Status)
		require.Equal(t, conditions.ReasonMetricAgentDaemonSetNotReady, cond.Reason)

		mock.AssertExpectationsForObjects(t, agentProberMock)
	})

	t.Run("metric agent daemonset is ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().PrometheusInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		agentProberMock := &mocks.DaemonSetProber{}
		fakeAgentName := types.NamespacedName{Name: "agent", Namespace: "telemetry"}
		agentProberMock.On("IsReady", mock.Anything, fakeAgentName).Return(true, nil)
		sut := Reconciler{
			Client: fakeClient,
			config: Config{Agent: otelcollector.AgentConfig{
				Config: otelcollector.Config{
					BaseName:  fakeAgentName.Name,
					Namespace: fakeAgentName.Namespace,
				},
			}},
			gatewayProber: gatewayProberStub,
			agentProber:   agentProberMock,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeMetricAgentReady)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeMetricAgentReady)
		require.Equal(t, metav1.ConditionTrue, cond.Status)
		require.Equal(t, conditions.ReasonMetricAgentDaemonSetReady, cond.Reason)

		mock.AssertExpectationsForObjects(t, agentProberMock)
	})
	//
	//t.Run("should add running condition if referenced secret exists and metric gateway deployment is ready", func(t *testing.T) {
	//	pipelineName := "pipeline"
	//	pipeline := &telemetryv1alpha1.MetricPipeline{
	//		ObjectMeta: metav1.ObjectMeta{
	//			Name: pipelineName,
	//		},
	//		Spec: telemetryv1alpha1.MetricPipelineSpec{
	//			Output: telemetryv1alpha1.MetricPipelineOutput{
	//				Otlp: &telemetryv1alpha1.OtlpOutput{
	//					Endpoint: telemetryv1alpha1.ValueType{
	//						ValueFrom: &telemetryv1alpha1.ValueFromSource{
	//							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
	//								Name:      "some-secret",
	//								Namespace: "some-namespace",
	//								Key:       "host",
	//							},
	//						},
	//					},
	//				},
	//			}},
	//	}
	//	secret := &corev1.Secret{
	//		TypeMeta: metav1.TypeMeta{},
	//		ObjectMeta: metav1.ObjectMeta{
	//			Name:      "some-secret",
	//			Namespace: "some-namespace",
	//		},
	//		Data: map[string][]byte{"host": nil},
	//	}
	//	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline, secret).WithStatusSubresource(pipeline).Build()
	//
	//	proberStub := &mocks.DeploymentProber{}
	//	proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)
	//
	//	sut := Reconciler{
	//		Client: fakeClient,
	//		config: Config{Gateway: otelcollector.GatewayConfig{
	//			Config: otelcollector.Config{BaseName: "metric-gateway"},
	//		}},
	//		gatewayProber: proberStub,
	//	}
	//
	//	err := sut.updateStatus(context.Background(), pipeline.Name, true)
	//	require.NoError(t, err)
	//
	//	var updatedPipeline telemetryv1alpha1.MetricPipeline
	//	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
	//	require.Len(t, updatedPipeline.Status.Conditions, 1)
	//	require.Equal(t, updatedPipeline.Status.Conditions[0].Type, telemetryv1alpha1.MetricPipelineRunning)
	//	require.Equal(t, updatedPipeline.Status.Conditions[0].Reason, conditions.ReasonMetricGatewayDeploymentReady)
	//})
	//
	//t.Run("should add pending condition if waiting for lock", func(t *testing.T) {
	//	pipelineName := "pipeline"
	//	pipeline := &telemetryv1alpha1.MetricPipeline{
	//		ObjectMeta: metav1.ObjectMeta{
	//			Name: pipelineName,
	//		},
	//		Spec: telemetryv1alpha1.MetricPipelineSpec{
	//			Output: telemetryv1alpha1.MetricPipelineOutput{
	//				Otlp: &telemetryv1alpha1.OtlpOutput{
	//					Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
	//				},
	//			}},
	//	}
	//	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()
	//
	//	proberStub := &mocks.DeploymentProber{}
	//	proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)
	//
	//	sut := Reconciler{
	//		Client: fakeClient,
	//		config: Config{Gateway: otelcollector.GatewayConfig{
	//			Config: otelcollector.Config{BaseName: "metric-gateway"},
	//		}},
	//		gatewayProber: proberStub,
	//	}
	//	err := sut.updateStatus(context.Background(), pipeline.Name, false)
	//	require.NoError(t, err)
	//
	//	var updatedPipeline telemetryv1alpha1.MetricPipeline
	//	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
	//	require.Len(t, updatedPipeline.Status.Conditions, 1)
	//	require.Equal(t, updatedPipeline.Status.Conditions[0].Type, telemetryv1alpha1.MetricPipelinePending)
	//	require.Equal(t, updatedPipeline.Status.Conditions[0].Reason, conditions.ReasonWaitingForLock)
	//})
	//
	//t.Run("should add pending condition if acquired lock but metric gateway is not ready", func(t *testing.T) {
	//	pipelineName := "pipeline"
	//	pipeline := &telemetryv1alpha1.MetricPipeline{
	//		ObjectMeta: metav1.ObjectMeta{
	//			Name: pipelineName,
	//		},
	//		Spec: telemetryv1alpha1.MetricPipelineSpec{
	//			Output: telemetryv1alpha1.MetricPipelineOutput{
	//				Otlp: &telemetryv1alpha1.OtlpOutput{
	//					Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
	//				},
	//			}},
	//	}
	//	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()
	//
	//	proberStub := &mocks.DeploymentProber{}
	//	proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)
	//
	//	sut := Reconciler{
	//		Client: fakeClient,
	//		config: Config{Gateway: otelcollector.GatewayConfig{
	//			Config: otelcollector.Config{BaseName: "metric-gateway"},
	//		}},
	//		gatewayProber: proberStub,
	//	}
	//	err := sut.updateStatus(context.Background(), pipeline.Name, false)
	//	require.NoError(t, err)
	//	err = sut.updateStatus(context.Background(), pipeline.Name, true)
	//	require.NoError(t, err)
	//
	//	var updatedPipeline telemetryv1alpha1.MetricPipeline
	//	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
	//	require.Len(t, updatedPipeline.Status.Conditions, 1)
	//	require.Equal(t, updatedPipeline.Status.Conditions[0].Type, telemetryv1alpha1.MetricPipelinePending)
	//	require.Equal(t, updatedPipeline.Status.Conditions[0].Reason, conditions.ReasonMetricGatewayDeploymentNotReady)
	//})
	//
	//t.Run("should add pending condition if metric gateway deployment is not ready but metric agent is ready", func(t *testing.T) {
	//	pipelineName := "pipeline"
	//	pipeline := &telemetryv1alpha1.MetricPipeline{
	//		ObjectMeta: metav1.ObjectMeta{
	//			Name: pipelineName,
	//		},
	//		Spec: telemetryv1alpha1.MetricPipelineSpec{
	//			Output: telemetryv1alpha1.MetricPipelineOutput{
	//				Otlp: &telemetryv1alpha1.OtlpOutput{
	//					Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
	//				},
	//			},
	//			Input: telemetryv1alpha1.MetricPipelineInput{
	//				Runtime: telemetryv1alpha1.MetricPipelineRuntimeInput{
	//					Enabled: pointer.Bool(true),
	//				},
	//				Prometheus: telemetryv1alpha1.MetricPipelinePrometheusInput{
	//					Enabled: pointer.Bool(false),
	//				},
	//				Istio: telemetryv1alpha1.MetricPipelineIstioInput{
	//					Enabled: pointer.Bool(false),
	//				},
	//			},
	//		},
	//	}
	//	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()
	//
	//	gatewayProberStub := &mocks.DeploymentProber{}
	//	gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)
	//
	//	agentProberStub := &mocks.DaemonSetProber{}
	//	agentProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)
	//	sut := Reconciler{
	//		Client: fakeClient,
	//		config: Config{Gateway: otelcollector.GatewayConfig{
	//			Config: otelcollector.Config{BaseName: "metric-gateway"},
	//		}},
	//		gatewayProber: gatewayProberStub,
	//		agentProber:   agentProberStub,
	//	}
	//	err := sut.updateStatus(context.Background(), pipeline.Name, true)
	//	require.NoError(t, err)
	//
	//	var updatedPipeline telemetryv1alpha1.MetricPipeline
	//	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
	//	require.Len(t, updatedPipeline.Status.Conditions, 1)
	//	require.Equal(t, updatedPipeline.Status.Conditions[0].Type, telemetryv1alpha1.MetricPipelinePending)
	//	require.Equal(t, updatedPipeline.Status.Conditions[0].Reason, conditions.ReasonMetricGatewayDeploymentNotReady)
	//})
}
