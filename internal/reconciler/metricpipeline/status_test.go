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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestUpdateStatus(t *testing.T) {
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
				BaseName:  "gateway",
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

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			gatewayProber:      gatewayProberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeGatewayHealthy)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeGatewayHealthy)
		require.Equal(t, metav1.ConditionFalse, cond.Status)
		require.Equal(t, conditions.ReasonGatewayNotReady, cond.Reason)

		mock.AssertExpectationsForObjects(t, gatewayProberStub)
	})

	t.Run("metric gateway prober fails", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberMock := &mocks.DeploymentProber{}
		gatewayProberMock.On("IsReady", mock.Anything, mock.Anything).Return(false, assert.AnError)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			gatewayProber:      gatewayProberMock,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeGatewayHealthy)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeGatewayHealthy)
		require.Equal(t, metav1.ConditionFalse, cond.Status)
		require.Equal(t, conditions.ReasonGatewayNotReady, cond.Reason)

		mock.AssertExpectationsForObjects(t, gatewayProberMock)
	})

	t.Run("metric gateway deployment is ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberMock := &mocks.DeploymentProber{}
		gatewayProberMock.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			gatewayProber:      gatewayProberMock,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeGatewayHealthy)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeGatewayHealthy)
		require.Equal(t, metav1.ConditionTrue, cond.Status)
		require.Equal(t, conditions.ReasonGatewayReady, cond.Reason)

		mock.AssertExpectationsForObjects(t, gatewayProberMock)
	})

	t.Run("metric agent daemonset is not ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		agentProberMock := &mocks.DaemonSetProber{}
		agentProberMock.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			gatewayProber:      gatewayProberStub,
			agentProber:        agentProberMock,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeAgentHealthy)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeAgentHealthy)
		require.Equal(t, metav1.ConditionFalse, cond.Status)
		require.Equal(t, conditions.ReasonAgentNotReady, cond.Reason)

		mock.AssertExpectationsForObjects(t, agentProberMock)
	})

	t.Run("metric agent prober fails", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		agentProberMock := &mocks.DaemonSetProber{}
		agentProberMock.On("IsReady", mock.Anything, mock.Anything).Return(false, assert.AnError)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			gatewayProber:      gatewayProberStub,
			agentProber:        agentProberMock,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeAgentHealthy)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeAgentHealthy)
		require.Equal(t, metav1.ConditionFalse, cond.Status)
		require.Equal(t, conditions.ReasonAgentNotReady, cond.Reason)

		mock.AssertExpectationsForObjects(t, agentProberMock)
	})

	t.Run("metric agent daemonset is ready", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		agentProberMock := &mocks.DaemonSetProber{}
		agentProberMock.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			gatewayProber:      gatewayProberStub,
			agentProber:        agentProberMock,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeAgentHealthy)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeAgentHealthy)
		require.Equal(t, metav1.ConditionTrue, cond.Status)
		require.Equal(t, conditions.ReasonAgentReady, cond.Reason)

		mock.AssertExpectationsForObjects(t, agentProberMock)
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

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			gatewayProber:      gatewayProberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
		require.Equal(t, metav1.ConditionTrue, cond.Status)
		require.Equal(t, conditions.ReasonAgentGatewayConfigured, cond.Reason)
	})

	t.Run("referenced secret missing", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			gatewayProber:      gatewayProberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
		require.Equal(t, metav1.ConditionFalse, cond.Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, cond.Reason)
	})

	t.Run("max pipelines exceeded", func(t *testing.T) {
		pipeline := testutils.NewMetricPipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(resourcelock.ErrLockInUse)

		gatewayProberStub := &mocks.DeploymentProber{}
		gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			gatewayProber:      gatewayProberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.Error(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
		require.Equal(t, metav1.ConditionFalse, cond.Status)
		require.Equal(t, conditions.ReasonMaxPipelinesExceeded, cond.Reason)
	})

	t.Run("flow healthy", func(t *testing.T) {
		tests := []struct {
			name           string
			probe          prober.OTelPipelineProbeResult
			probeErr       error
			expectedStatus metav1.ConditionStatus
			expectedReason string
		}{
			{
				name:           "prober fails",
				probeErr:       assert.AnError,
				expectedStatus: metav1.ConditionUnknown,
				expectedReason: conditions.ReasonSelfMonProbingNotReachable,
			},
			{
				name: "healthy",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
				},
				expectedStatus: metav1.ConditionTrue,
				expectedReason: conditions.ReasonSelfMonFlowHealthy,
			},
			{
				name: "throttling",
				probe: prober.OTelPipelineProbeResult{
					Throttling: true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonSelfMonGatewayThrottling,
			},
			{
				name: "buffer filling up",
				probe: prober.OTelPipelineProbeResult{
					QueueAlmostFull: true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonSelfMonBufferFillingUp,
			},
			{
				name: "buffer filling up shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					QueueAlmostFull: true,
					Throttling:      true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonSelfMonBufferFillingUp,
			},
			{
				name: "some data dropped",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonSelfMonSomeDataDropped,
			},
			{
				name: "some data dropped shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
					Throttling:          true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonSelfMonSomeDataDropped,
			},
			{
				name: "all data dropped",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonSelfMonAllDataDropped,
			},
			{
				name: "all data dropped shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
					Throttling:          true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonSelfMonAllDataDropped,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewMetricPipelineBuilder().Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				pipelineLockStub := &mocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

				gatewayProberStub := &mocks.DeploymentProber{}
				gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

				flowHealthProberStub := &mocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

				sut := Reconciler{
					Client:                   fakeClient,
					config:                   testConfig,
					pipelineLock:             pipelineLockStub,
					gatewayProber:            gatewayProberStub,
					flowHealthProbingEnabled: true,
					flowHealthProber:         flowHealthProberStub,
					overridesHandler:         overridesHandlerStub,
					istioStatusChecker:       istioStatusCheckerStub,
				}
				_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.MetricPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeFlowHealthy)
				require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeFlowHealthy)
				require.Equal(t, tt.expectedStatus, cond.Status)
				require.Equal(t, tt.expectedReason, cond.Reason)
			})
		}
	})
	t.Run("tls conditions", func(t *testing.T) {
		tests := []struct {
			name           string
			tlsCertErr     error
			expectedStatus metav1.ConditionStatus
			expectedReason string
		}{
			{
				name:           "cert expired",
				tlsCertErr:     &tlscert.CertExpiredError{Expiry: time.Now().Add(-time.Hour)},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonTLSCertificateExpired,
			},
			{
				name:           "cert about to expire",
				tlsCertErr:     &tlscert.CertAboutToExpireError{Expiry: time.Now().Add(7 * 24 * time.Hour)},
				expectedStatus: metav1.ConditionTrue,
				expectedReason: conditions.ReasonTLSCertificateAboutToExpire,
			},
			{
				name:           "cert decode failed",
				tlsCertErr:     tlscert.ErrCertDecodeFailed,
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonTLSCertificateInvalid,
			},
			{
				name:           "key decode failed",
				tlsCertErr:     tlscert.ErrKeyDecodeFailed,
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonTLSCertificateInvalid,
			},
			{
				name:           "key parse failed",
				tlsCertErr:     tlscert.ErrKeyParseFailed,
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonTLSCertificateInvalid,
			},
			{
				name:           "cert parse failed",
				tlsCertErr:     tlscert.ErrCertParseFailed,
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonTLSCertificateInvalid,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewMetricPipelineBuilder().WithOTLPOutput(testutils.OTLPClientTLS("ca", "fooCert", "fooKey")).Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				pipelineLockStub := &mocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

				gatewayProberStub := &mocks.DeploymentProber{}
				gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)
				tlsStub := &mocks.TLSCertValidator{}
				tlsStub.On("ValidateCertificate", mock.Anything, mock.Anything, mock.Anything).Return(tt.tlsCertErr)

				sut := Reconciler{
					Client:             fakeClient,
					config:             testConfig,
					pipelineLock:       pipelineLockStub,
					gatewayProber:      gatewayProberStub,
					tlsCertValidator:   tlsStub,
					overridesHandler:   overridesHandlerStub,
					istioStatusChecker: istioStatusCheckerStub,
				}
				_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.MetricPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
				cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
				require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
				require.Equal(t, tt.expectedStatus, cond.Status)
				require.Equal(t, tt.expectedReason, cond.Reason)
			})
		}

	})
}
