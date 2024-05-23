package logpipeline

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
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
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
		DaemonSet:             types.NamespacedName{Name: "test-telemetry-fluent-bit", Namespace: "default"},
		SectionsConfigMap:     types.NamespacedName{Name: "test-telemetry-fluent-bit-sections", Namespace: "default"},
		FilesConfigMap:        types.NamespacedName{Name: "test-telemetry-fluent-bit-files", Namespace: "default"},
		LuaConfigMap:          types.NamespacedName{Name: "test-telemetry-fluent-bit-lua", Namespace: "default"},
		ParsersConfigMap:      types.NamespacedName{Name: "test-telemetry-fluent-bit-parsers", Namespace: "default"},
		EnvSecret:             types.NamespacedName{Name: "test-telemetry-fluent-bit-env", Namespace: "default"},
		OutputTLSConfigSecret: types.NamespacedName{Name: "test-telemetry-fluent-bit-output-tls-config", Namespace: "default"},
		DaemonSetConfig: fluentbit.DaemonSetConfig{
			FluentBitImage: "fluent/bit:latest",
			ExporterImage:  "exporter:latest",
		},
	}

	t.Run("should set status UnsupportedMode true if contains custom plugin", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").WithCustomFilter("Name grep").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
			syncer: syncer{
				Client: fakeClient,
				config: testConfig,
			},
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		require.True(t, updatedPipeline.Status.UnsupportedMode)
	})

	t.Run("should set status UnsupportedMode false if does not contains custom plugin", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
			syncer: syncer{
				Client: fakeClient,
				config: testConfig,
			},
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		require.False(t, updatedPipeline.Status.UnsupportedMode)
	})

	t.Run("log agent is not ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
			syncer: syncer{
				Client: fakeClient,
				config: testConfig,
			},
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionFalse,
			conditions.ReasonAgentNotReady,
			"Fluent Bit agent DaemonSet is not ready",
		)

		requireEndsWithLegacyPendingCondition(t, updatedPipeline,
			conditions.ReasonFluentBitDSNotReady,
			"[NOTE: The \"Pending\" type is deprecated] Fluent Bit DaemonSet is not ready")

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.NoError(t, err, "sections configmap must exist")
		require.Contains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must contain pipeline name")
	})

	t.Run("log agent is ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
			syncer: syncer{
				Client: fakeClient,
				config: testConfig,
			},
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionTrue,
			conditions.ReasonAgentReady,
			"Fluent Bit agent DaemonSet is ready",
		)

		requireEndsWithLegacyRunningCondition(t, updatedPipeline,
			conditions.ReasonFluentBitDSReady,
			"[NOTE: The \"Running\" type is deprecated] Fluent Bit DaemonSet is ready")

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.NoError(t, err, "sections configmap must exist")
		require.Contains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must contain pipeline name")
	})

	t.Run("referenced secret missing", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().
			WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
			WithHTTPOutput(testutils.HTTPHostFromSecret("some-secret", "some-namespace", "host")).
			Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
			syncer: syncer{
				Client: fakeClient,
				config: testConfig,
			},
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonReferencedSecretMissing,
			"One or more referenced Secrets are missing",
		)

		requireEndsWithLegacyPendingCondition(t, updatedPipeline,
			conditions.ReasonReferencedSecretMissing,
			"[NOTE: The \"Pending\" type is deprecated] One or more referenced Secrets are missing")

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.NoError(t, err, "sections configmap must exist")
		require.NotContains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must not contain pipeline name")
	})

	t.Run("referenced secret exists", func(t *testing.T) {
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: "some-namespace",
			},
			Data: map[string][]byte{"host": nil},
		}
		pipeline := testutils.NewLogPipelineBuilder().
			WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
			WithHTTPOutput(testutils.HTTPHostFromSecret("some-secret", "some-namespace", "host")).
			Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline, secret).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
			syncer: syncer{
				Client: fakeClient,
				config: testConfig,
			},
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionTrue,
			conditions.ReasonAgentConfigured,
			"Fluent Bit agent successfully configured",
		)

		requireEndsWithLegacyRunningCondition(t, updatedPipeline,
			conditions.ReasonFluentBitDSReady,
			"[NOTE: The \"Running\" type is deprecated] Fluent Bit DaemonSet is ready")

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.NoError(t, err, "sections configmap must exist")
		require.Contains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must contain pipeline name")
	})

	t.Run("loki output is defined", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").WithLokiOutput().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
			syncer: syncer{
				Client: fakeClient,
				config: testConfig,
			},
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonUnsupportedLokiOutput,
			"grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow https://kyma-project.io/#/telemetry-manager/user/integration/loki/README",
		)

		requireEndsWithLegacyPendingCondition(t, updatedPipeline,
			conditions.ReasonUnsupportedLokiOutput,
			"[NOTE: The \"Pending\" type is deprecated] grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow https://kyma-project.io/#/telemetry-manager/user/integration/loki/README")

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.NoError(t, err, "sections configmap must exist")
		require.NotContains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must not contain pipeline name")
	})

	t.Run("flow healthy", func(t *testing.T) {
		tests := []struct {
			name            string
			probe           prober.LogPipelineProbeResult
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
				expectedMessage: "Self monitoring probing failed for unknown reasons",
			},
			{
				name: "healthy",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
				},
				expectedStatus:  metav1.ConditionTrue,
				expectedReason:  conditions.ReasonSelfMonFlowHealthy,
				expectedMessage: "No problems detected in the log flow",
			},
			{
				name: "buffer filling up",
				probe: prober.LogPipelineProbeResult{
					BufferFillingUp: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
				expectedMessage: "Buffer nearing capacity: incoming log rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=influx-capacity-reaching-its-limit",
			},
			{
				name: "no logs delivered",
				probe: prober.LogPipelineProbeResult{
					NoLogsDelivered: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonNoLogsDelivered,
				expectedMessage: "No logs delivered to backend. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=logs-not-arriving-at-the-destination",
			},
			{
				name: "no logs delivered shadows other problems",
				probe: prober.LogPipelineProbeResult{
					NoLogsDelivered: true,
					BufferFillingUp: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonNoLogsDelivered,
				expectedMessage: "No logs delivered to backend. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=logs-not-arriving-at-the-destination",
			},
			{
				name: "some data dropped",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Some logs dropped: backend unreachable or rejecting. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=influx-capacity-reaching-its-limit",
			},
			{
				name: "some data dropped shadows other problems",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
					BufferFillingUp:     true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Some logs dropped: backend unreachable or rejecting. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=influx-capacity-reaching-its-limit",
			},
			{
				name: "all data dropped",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "All logs dropped: backend unreachable or rejecting. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=logs-not-arriving-at-the-destination",
			},
			{
				name: "all data dropped shadows other problems",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{
						AllDataDropped:  true,
						SomeDataDropped: true,
					},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "All logs dropped: backend unreachable or rejecting. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=logs-not-arriving-at-the-destination",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				agentProberStub := &mocks.DaemonSetProber{}
				agentProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

				flowHealthProberStub := &mocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

				sut := Reconciler{
					Client:                   fakeClient,
					config:                   testConfig,
					prober:                   agentProberStub,
					flowHealthProbingEnabled: true,
					flowHealthProber:         flowHealthProberStub,
					overridesHandler:         overridesHandlerStub,
					istioStatusChecker:       istioStatusCheckerStub,
					syncer: syncer{
						Client: fakeClient,
						config: testConfig,
					},
				}
				_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.LogPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				requireHasStatusCondition(t, updatedPipeline,
					conditions.TypeFlowHealthy,
					tt.expectedStatus,
					tt.expectedReason,
					tt.expectedMessage,
				)

				requireEndsWithLegacyRunningCondition(t, updatedPipeline,
					conditions.ReasonFluentBitDSReady,
					"[NOTE: The \"Running\" type is deprecated] Fluent Bit DaemonSet is ready")

				var cm corev1.ConfigMap
				err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
				require.NoError(t, err, "sections configmap must exist")
				require.Contains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must contain pipeline name")
			})
		}
	})

	t.Run("should remove running condition and set pending condition to true if fluent bit becomes not ready again", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().
			WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
			WithHTTPOutput(testutils.HTTPHostFromSecret("some-secret", "some-namespace", "host")).
			WithStatusConditions(
				metav1.Condition{
					Type:               conditions.TypeAgentHealthy,
					Status:             metav1.ConditionTrue,
					Reason:             conditions.ReasonAgentReady,
					LastTransitionTime: metav1.Now(),
				},
				metav1.Condition{
					Type:               conditions.TypeConfigurationGenerated,
					Status:             metav1.ConditionTrue,
					Reason:             conditions.ReasonAgentConfigured,
					LastTransitionTime: metav1.Now(),
				},
				metav1.Condition{
					Type:               conditions.TypePending,
					Status:             metav1.ConditionFalse,
					Reason:             conditions.ReasonFluentBitDSNotReady,
					LastTransitionTime: metav1.Now(),
				},
				metav1.Condition{
					Type:               conditions.TypeRunning,
					Status:             metav1.ConditionTrue,
					Reason:             conditions.ReasonFluentBitDSReady,
					LastTransitionTime: metav1.Now(),
				}).
			Build()
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: "some-namespace",
			},
			Data: map[string][]byte{"host": nil},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline, secret).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			overridesHandler:   overridesHandlerStub,
			istioStatusChecker: istioStatusCheckerStub,
			syncer: syncer{
				Client: fakeClient,
				config: testConfig,
			},
		}
		_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		requireEndsWithLegacyPendingCondition(t, updatedPipeline,
			conditions.ReasonFluentBitDSNotReady,
			"[NOTE: The \"Pending\" type is deprecated] Fluent Bit DaemonSet is not ready",
		)
	})

	t.Run("tls conditions", func(t *testing.T) {
		tests := []struct {
			name                  string
			tlsCertErr            error
			expectedStatus        metav1.ConditionStatus
			expectedReason        string
			expectedMessage       string
			expectAgentConfigured bool
		}{
			{
				name:            "cert expired",
				tlsCertErr:      &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC)},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSCertificateExpired,
				expectedMessage: "TLS certificate expired on 2020-11-01",
			},
			{
				name:                  "cert about to expire",
				tlsCertErr:            &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC)},
				expectedStatus:        metav1.ConditionTrue,
				expectedReason:        conditions.ReasonTLSCertificateAboutToExpire,
				expectedMessage:       "TLS certificate is about to expire, configured certificate is valid until 2024-11-01",
				expectAgentConfigured: true,
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
				pipeline := testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					WithHTTPOutput(testutils.HTTPClientTLS("ca", "fooCert", "fooKey")).
					Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				proberStub := &mocks.DaemonSetProber{}
				proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)
				tlsStub := &mocks.TLSCertValidator{}
				tlsStub.On("ValidateCertificate", mock.Anything, mock.Anything, mock.Anything).Return(tt.tlsCertErr)

				sut := Reconciler{
					Client:             fakeClient,
					config:             testConfig,
					prober:             proberStub,
					tlsCertValidator:   tlsStub,
					overridesHandler:   overridesHandlerStub,
					istioStatusChecker: istioStatusCheckerStub,
					syncer: syncer{
						Client: fakeClient,
						config: testConfig,
					},
				}
				_, err := sut.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.LogPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				requireHasStatusCondition(t, updatedPipeline,
					conditions.TypeConfigurationGenerated,
					tt.expectedStatus,
					tt.expectedReason,
					tt.expectedMessage,
				)

				var cm corev1.ConfigMap
				err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
				require.NoError(t, err, "sections configmap must exist")
				if !tt.expectAgentConfigured {
					require.NotContains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must not contain pipeline name")
				} else {
					require.Contains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must contain pipeline name")
				}
			})
		}
	})
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

func requireEndsWithLegacyPendingCondition(t *testing.T, pipeline telemetryv1alpha1.LogPipeline, reason, message string) {
	cond := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeRunning)
	require.Nil(t, cond, "running condition should not be present")

	require.NotEmpty(t, pipeline.Status.Conditions)

	condLen := len(pipeline.Status.Conditions)
	lastCond := pipeline.Status.Conditions[condLen-1]
	require.Equal(t, conditions.TypePending, lastCond.Type)
	require.Equal(t, metav1.ConditionTrue, lastCond.Status)
	require.Equal(t, reason, lastCond.Reason)
	require.Equal(t, message, lastCond.Message)
	require.Equal(t, pipeline.Generation, lastCond.ObservedGeneration)
	require.NotEmpty(t, lastCond.LastTransitionTime)
}

func requireEndsWithLegacyRunningCondition(t *testing.T, pipeline telemetryv1alpha1.LogPipeline, reason, message string) {
	require.Greater(t, len(pipeline.Status.Conditions), 1)

	condLen := len(pipeline.Status.Conditions)
	lastCond := pipeline.Status.Conditions[condLen-1]
	require.Equal(t, conditions.TypeRunning, lastCond.Type)
	require.Equal(t, metav1.ConditionTrue, lastCond.Status)
	require.Equal(t, reason, lastCond.Reason)
	require.Equal(t, message, lastCond.Message)
	require.Equal(t, pipeline.Generation, lastCond.ObservedGeneration)
	require.NotEmpty(t, lastCond.LastTransitionTime)

	prevCond := pipeline.Status.Conditions[condLen-2]
	require.Equal(t, conditions.TypePending, prevCond.Type)
	require.Equal(t, metav1.ConditionFalse, prevCond.Status)
}
