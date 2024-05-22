package tracepipeline

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
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

func TestStatus(t *testing.T) {
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

	t.Run("trace gateway deployment is not ready", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().WithName("pipeline").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionFalse,
			conditions.ReasonGatewayNotReady,
			conditions.MessageForTracePipeline(conditions.ReasonGatewayNotReady),
		)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentNotReady, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentNotReady)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("trace gateway deployment is ready", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().WithName("pipeline").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionTrue,
			conditions.ReasonGatewayReady,
			conditions.MessageForTracePipeline(conditions.ReasonGatewayReady),
		)

		conditionsSize := len(updatedPipeline.Status.Conditions)

		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-2]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionFalse, pendingCond.Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentNotReady, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentNotReady)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)

		runningCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypeRunning, runningCond.Type)
		require.Equal(t, metav1.ConditionTrue, runningCond.Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentReady, runningCond.Reason)
		runningCondMsg := conditions.RunningTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentReady)
		require.Equal(t, runningCondMsg, runningCond.Message)
		require.Equal(t, updatedPipeline.Generation, runningCond.ObservedGeneration)
		require.NotEmpty(t, runningCond.LastTransitionTime)
	})

	t.Run("referenced secret missing", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPEndpointFromSecret(
			"non-existing",
			"default",
			"endpoint")).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonReferencedSecretMissing,
			conditions.MessageForTracePipeline(conditions.ReasonReferencedSecretMissing),
		)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonReferencedSecretMissing)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
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

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionTrue,
			conditions.ReasonConfigurationGenerated,
			"",
		)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		runningCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypeRunning, runningCond.Type)
		require.Equal(t, metav1.ConditionTrue, runningCond.Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentReady, runningCond.Reason)
		runningCondMsg := conditions.RunningTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentReady)
		require.Equal(t, runningCondMsg, runningCond.Message)
		require.Equal(t, updatedPipeline.Generation, runningCond.ObservedGeneration)
		require.NotEmpty(t, runningCond.LastTransitionTime)
	})

	t.Run("max pipelines exceeded", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(resourcelock.ErrLockInUse)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.Error(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonMaxPipelinesExceeded,
			conditions.MessageForTracePipeline(conditions.ReasonMaxPipelinesExceeded),
		)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonMaxPipelinesExceeded, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonMaxPipelinesExceeded)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
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
				expectedReason: conditions.ReasonSelfMonFlowHealthy,
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
				pipeline := testutils.NewTracePipelineBuilder().Build()
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
					prober:                   gatewayProberStub,
					flowHealthProbingEnabled: true,
					flowHealthProber:         flowHealthProberStub,
					overridesHandler:         overridesHandlerStub,
					istioStatusChecker:       istioStatusCheckerStub,
				}
				_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.TracePipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				requireHasStatusCondition(t, updatedPipeline,
					conditions.TypeFlowHealthy,
					tt.expectedStatus,
					tt.expectedReason,
					conditions.MessageForTracePipeline(tt.expectedReason),
				)
			})
		}
	})

	t.Run("should remove running condition and set pending condition to true if trace gateway deployment becomes not ready again", func(t *testing.T) {
		pipeline := testutils.NewTracePipelineBuilder().
			WithOTLPOutput(testutils.OTLPEndpoint("localhost")).
			WithStatusConditions(
				metav1.Condition{
					Type:               conditions.TypeGatewayHealthy,
					Status:             metav1.ConditionTrue,
					Reason:             conditions.ReasonGatewayReady,
					Message:            conditions.MessageForTracePipeline(conditions.ReasonGatewayReady),
					LastTransitionTime: metav1.Now(),
				},
				metav1.Condition{
					Type:               conditions.TypeConfigurationGenerated,
					Status:             metav1.ConditionTrue,
					Reason:             conditions.TypeConfigurationGenerated,
					Message:            conditions.MessageForTracePipeline(conditions.TypeConfigurationGenerated),
					LastTransitionTime: metav1.Now(),
				},
				metav1.Condition{
					Type:               conditions.TypePending,
					Status:             metav1.ConditionFalse,
					Reason:             conditions.ReasonTraceGatewayDeploymentNotReady,
					Message:            conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentNotReady),
					LastTransitionTime: metav1.Now(),
				},
				metav1.Condition{
					Type:               conditions.TypeRunning,
					Status:             metav1.ConditionTrue,
					Reason:             conditions.ReasonTraceGatewayDeploymentReady,
					Message:            conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentReady),
					LastTransitionTime: metav1.Now(),
				}).
			Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			pipelineLock:       pipelineLockStub,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentNotReady, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentNotReady)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("tls conditions", func(t *testing.T) {
		tests := []struct {
			name            string
			tlsCertErr      error
			expectedStatus  metav1.ConditionStatus
			expectedReason  string
			expectedMessage string
		}{
			{
				name:            "cert expired",
				tlsCertErr:      &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC)},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSCertificateExpired,
				expectedMessage: "TLS certificate expired on 2020-11-01",
			},
			{
				name:            "cert about to expire",
				tlsCertErr:      &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC)},
				expectedStatus:  metav1.ConditionTrue,
				expectedReason:  conditions.ReasonTLSCertificateAboutToExpire,
				expectedMessage: "TLS certificate is about to expire, configured certificate is valid until 2024-11-01",
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
				pipeline := testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPClientTLS("ca", "fooCert", "fooKey")).Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				pipelineLockStub := &mocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

				proberStub := &mocks.DeploymentProber{}
				proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

				tlsStub := &mocks.TLSCertValidator{}
				tlsStub.On("ValidateCertificate", mock.Anything, mock.Anything, mock.Anything).Return(tt.tlsCertErr)

				sut := Reconciler{
					Client:             fakeClient,
					config:             testConfig,
					pipelineLock:       pipelineLockStub,
					prober:             proberStub,
					tlsCertValidator:   tlsStub,
					overridesHandler:   overridesHandlerStub,
					istioStatusChecker: istioStatusCheckerStub,
				}
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

func requireHasLegacyRunningCondition(t *testing.T, pipeline telemetryv1alpha1.TracePipeline, condType string, status metav1.ConditionStatus, reason, message string) {
	cond := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeRunning)
	require.NotNil(t, cond, "could not find condition of type %s", condType)
	require.Equal(t, status, cond.Status)
	require.Equal(t, reason, cond.Reason)
	require.Equal(t, message, cond.Message)
	require.Equal(t, pipeline.Generation, cond.ObservedGeneration)
	require.NotEmpty(t, cond.LastTransitionTime)
}
