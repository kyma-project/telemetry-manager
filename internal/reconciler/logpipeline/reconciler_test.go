package logpipeline

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
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

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			flowHealthProber:   flowHealthProberStub,
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

		require.True(t, *updatedPipeline.Status.UnsupportedMode)
	})

	t.Run("should set status UnsupportedMode false if does not contains custom plugin", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			flowHealthProber:   flowHealthProberStub,
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

		require.False(t, *updatedPipeline.Status.UnsupportedMode)
	})

	t.Run("log agent is not ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			flowHealthProber:   flowHealthProberStub,
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

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			flowHealthProber:   flowHealthProberStub,
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
			"[NOTE: The \"Running\" type is deprecated] Fluent Bit DaemonSet is ready")

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.NoError(t, err, "sections configmap must exist")
		require.Contains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must contain pipeline name")
	})

	t.Run("log agent prober fails", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, assert.AnError)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			flowHealthProber:   flowHealthProberStub,
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

	t.Run("referenced secret missing", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().
			WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
			WithHTTPOutput(testutils.HTTPHostFromSecret("some-secret", "some-namespace", "host")).
			Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			flowHealthProber:   flowHealthProberStub,
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
			"One or more referenced Secrets are missing, first finding is: secret 'some-secret' of namespace 'some-namespace'",
		)

		requireEndsWithLegacyPendingCondition(t, updatedPipeline,
			conditions.ReasonReferencedSecretMissing,
			"[NOTE: The \"Pending\" type is deprecated] One or more referenced Secrets are missing, first finding is: secret 'some-secret' of namespace 'some-namespace'")

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeFlowHealthy,
			metav1.ConditionFalse,
			conditions.ReasonSelfMonConfigNotGenerated,
			"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Fluent Bit agent. Check the 'ConfigurationGenerated' condition for more details",
		)

		name := types.NamespacedName{Name: testConfig.DaemonSet.Name, Namespace: testConfig.DaemonSet.Namespace}

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.Error(t, err, "sections configmap should not exist")

		var cmLua corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.LuaConfigMap, &cmLua)
		require.Error(t, err, "lua configmap should not exist")

		var cmParser corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.ParsersConfigMap, &cmParser)
		require.Error(t, err, "parser configmap should not exist")

		var serviceAccount corev1.ServiceAccount
		err = fakeClient.Get(context.Background(), name, &serviceAccount)
		require.Error(t, err, "service account should not exist")

		var clusterRole rbacv1.ClusterRole
		err = fakeClient.Get(context.Background(), name, &clusterRole)
		require.Error(t, err, "clusterrole should not exist")

		var clusterRoleBinding rbacv1.ClusterRoleBinding
		err = fakeClient.Get(context.Background(), name, &clusterRoleBinding)
		require.Error(t, err, "clusterrolebinding should not exist")

		var daemonSet appsv1.DaemonSet
		err = fakeClient.Get(context.Background(), name, &daemonSet)
		require.Error(t, err, "daemonset should not exist")

		var networkPolicy networkingv1.NetworkPolicy
		err = fakeClient.Get(context.Background(), name, &networkPolicy)
		require.Error(t, err, "network policy should not exist")
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

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			flowHealthProber:   flowHealthProberStub,
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
			"LogPipeline specification is successfully applied to the configuration of Fluent Bit agent",
		)

		requireEndsWithLegacyRunningCondition(t, updatedPipeline,
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

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			flowHealthProber:   flowHealthProberStub,
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

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeFlowHealthy,
			metav1.ConditionFalse,
			conditions.ReasonSelfMonConfigNotGenerated,
			"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Fluent Bit agent. Check the 'ConfigurationGenerated' condition for more details",
		)

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.Error(t, err, "sections configmap should not exist")
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
				expectedMessage: "Could not determine the health of the telemetry flow because the self monitor probing failed",
			},
			{
				name: "healthy",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
				},
				expectedStatus:  metav1.ConditionTrue,
				expectedReason:  conditions.ReasonSelfMonFlowHealthy,
				expectedMessage: "No problems detected in the telemetry flow",
			},
			{
				name: "buffer filling up",
				probe: prober.LogPipelineProbeResult{
					BufferFillingUp: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
				expectedMessage: "Buffer nearing capacity. Incoming log rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=agent-buffer-filling-up",
			},
			{
				name: "no logs delivered",
				probe: prober.LogPipelineProbeResult{
					NoLogsDelivered: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonNoLogsDelivered,
				expectedMessage: "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
			},
			{
				name: "no logs delivered shadows other problems",
				probe: prober.LogPipelineProbeResult{
					NoLogsDelivered: true,
					BufferFillingUp: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonNoLogsDelivered,
				expectedMessage: "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
			},
			{
				name: "some data dropped",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=not-all-logs-arrive-at-the-backend",
			},
			{
				name: "some data dropped shadows other problems",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
					BufferFillingUp:     true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=not-all-logs-arrive-at-the-backend",
			},
			{
				name: "all data dropped",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
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
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
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
					Client:             fakeClient,
					config:             testConfig,
					prober:             agentProberStub,
					flowHealthProber:   flowHealthProberStub,
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
					conditions.TypeFlowHealthy,
					tt.expectedStatus,
					tt.expectedReason,
					tt.expectedMessage,
				)

				requireEndsWithLegacyRunningCondition(t, updatedPipeline,
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

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		sut := Reconciler{
			Client:             fakeClient,
			config:             testConfig,
			prober:             proberStub,
			flowHealthProber:   flowHealthProberStub,
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
			name                    string
			tlsCertErr              error
			expectedStatus          metav1.ConditionStatus
			expectedReason          string
			expectedMessage         string
			expectedLegacyCondition string
			expectAgentConfigured   bool
		}{
			{
				name:                    "cert expired",
				tlsCertErr:              &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC)},
				expectedStatus:          metav1.ConditionFalse,
				expectedReason:          conditions.ReasonTLSCertificateExpired,
				expectedMessage:         "TLS certificate expired on 2020-11-01",
				expectedLegacyCondition: conditions.TypePending,
			},
			{
				name:                    "cert about to expire",
				tlsCertErr:              &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC)},
				expectedStatus:          metav1.ConditionTrue,
				expectedReason:          conditions.ReasonTLSCertificateAboutToExpire,
				expectedMessage:         "TLS certificate is about to expire, configured certificate is valid until 2024-11-01",
				expectedLegacyCondition: conditions.TypeRunning,
				expectAgentConfigured:   true,
			},
			{
				name:                    "ca expired",
				tlsCertErr:              &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
				expectedStatus:          metav1.ConditionFalse,
				expectedReason:          conditions.ReasonTLSCertificateExpired,
				expectedMessage:         "TLS CA certificate expired on 2020-11-01",
				expectedLegacyCondition: conditions.TypePending,
			},
			{
				name:                    "ca about to expire",
				tlsCertErr:              &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
				expectedStatus:          metav1.ConditionTrue,
				expectedReason:          conditions.ReasonTLSCertificateAboutToExpire,
				expectedMessage:         "TLS CA certificate is about to expire, configured certificate is valid until 2024-11-01",
				expectedLegacyCondition: conditions.TypeRunning,
				expectAgentConfigured:   true,
			},
			{
				name:                    "cert decode failed",
				tlsCertErr:              tlscert.ErrCertDecodeFailed,
				expectedStatus:          metav1.ConditionFalse,
				expectedReason:          conditions.ReasonTLSConfigurationInvalid,
				expectedMessage:         "TLS configuration invalid: failed to decode PEM block containing certificate",
				expectedLegacyCondition: conditions.TypePending,
			},
			{
				name:                    "key decode failed",
				tlsCertErr:              tlscert.ErrKeyDecodeFailed,
				expectedStatus:          metav1.ConditionFalse,
				expectedReason:          conditions.ReasonTLSConfigurationInvalid,
				expectedMessage:         "TLS configuration invalid: failed to decode PEM block containing private key",
				expectedLegacyCondition: conditions.TypePending,
			},
			{
				name:                    "key parse failed",
				tlsCertErr:              tlscert.ErrKeyParseFailed,
				expectedStatus:          metav1.ConditionFalse,
				expectedReason:          conditions.ReasonTLSConfigurationInvalid,
				expectedMessage:         "TLS configuration invalid: failed to parse private key",
				expectedLegacyCondition: conditions.TypePending,
			},
			{
				name:                    "cert parse failed",
				tlsCertErr:              tlscert.ErrCertParseFailed,
				expectedStatus:          metav1.ConditionFalse,
				expectedReason:          conditions.ReasonTLSConfigurationInvalid,
				expectedMessage:         "TLS configuration invalid: failed to parse certificate",
				expectedLegacyCondition: conditions.TypePending,
			},
			{
				name:                    "cert and key mismatch",
				tlsCertErr:              tlscert.ErrInvalidCertificateKeyPair,
				expectedStatus:          metav1.ConditionFalse,
				expectedReason:          conditions.ReasonTLSConfigurationInvalid,
				expectedMessage:         "TLS configuration invalid: certificate and private key do not match",
				expectedLegacyCondition: conditions.TypePending,
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

				flowHealthProberStub := &mocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

				sut := Reconciler{
					Client:             fakeClient,
					config:             testConfig,
					prober:             proberStub,
					flowHealthProber:   flowHealthProberStub,
					tlsCertValidator:   stubs.NewTLSCertValidator(tt.tlsCertErr),
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

				if tt.expectedStatus == metav1.ConditionFalse {
					requireHasStatusCondition(t, updatedPipeline,
						conditions.TypeFlowHealthy,
						metav1.ConditionFalse,
						conditions.ReasonSelfMonConfigNotGenerated,
						"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Fluent Bit agent. Check the 'ConfigurationGenerated' condition for more details",
					)
				}

				if tt.expectedLegacyCondition == conditions.TypePending {
					expectedLegacyMessage := conditions.PendingTypeDeprecationMsg + tt.expectedMessage
					requireEndsWithLegacyPendingCondition(t, updatedPipeline, tt.expectedReason, expectedLegacyMessage)
				} else {
					expectedLegacyMessage := conditions.RunningTypeDeprecationMsg + conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSReady)
					requireEndsWithLegacyRunningCondition(t, updatedPipeline, expectedLegacyMessage)
				}

				var cm corev1.ConfigMap
				err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
				if !tt.expectAgentConfigured {
					require.Error(t, err, "sections configmap should not exist")
				} else {
					require.NoError(t, err, "sections configmap must exist")
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

func requireEndsWithLegacyRunningCondition(t *testing.T, pipeline telemetryv1alpha1.LogPipeline, message string) {
	require.Greater(t, len(pipeline.Status.Conditions), 1)

	condLen := len(pipeline.Status.Conditions)
	lastCond := pipeline.Status.Conditions[condLen-1]
	require.Equal(t, conditions.TypeRunning, lastCond.Type)
	require.Equal(t, metav1.ConditionTrue, lastCond.Status)
	require.Equal(t, conditions.ReasonFluentBitDSReady, lastCond.Reason)
	require.Equal(t, message, lastCond.Message)
	require.Equal(t, pipeline.Generation, lastCond.ObservedGeneration)
	require.NotEmpty(t, lastCond.LastTransitionTime)

	prevCond := pipeline.Status.Conditions[condLen-2]
	require.Equal(t, conditions.TypePending, prevCond.Type)
	require.Equal(t, metav1.ConditionFalse, prevCond.Status)
}

func TestCalculateChecksum(t *testing.T) {
	config := Config{
		DaemonSet: types.NamespacedName{
			Namespace: "default",
			Name:      "daemonset",
		},
		SectionsConfigMap: types.NamespacedName{
			Namespace: "default",
			Name:      "sections",
		},
		FilesConfigMap: types.NamespacedName{
			Namespace: "default",
			Name:      "files",
		},
		LuaConfigMap: types.NamespacedName{
			Namespace: "default",
			Name:      "lua",
		},
		ParsersConfigMap: types.NamespacedName{
			Namespace: "default",
			Name:      "parsers",
		},
		EnvSecret: types.NamespacedName{
			Namespace: "default",
			Name:      "env",
		},
		OutputTLSConfigSecret: types.NamespacedName{
			Namespace: "default",
			Name:      "tls",
		},
	}
	dsConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.DaemonSet.Name,
			Namespace: config.DaemonSet.Namespace,
		},
		Data: map[string]string{
			"a": "b",
		},
	}
	sectionsConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.SectionsConfigMap.Name,
			Namespace: config.SectionsConfigMap.Namespace,
		},
		Data: map[string]string{
			"a": "b",
		},
	}
	filesConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.FilesConfigMap.Name,
			Namespace: config.FilesConfigMap.Namespace,
		},
		Data: map[string]string{
			"a": "b",
		},
	}
	luaConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.LuaConfigMap.Name,
			Namespace: config.LuaConfigMap.Namespace,
		},
		Data: map[string]string{
			"a": "b",
		},
	}
	parsersConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ParsersConfigMap.Name,
			Namespace: config.ParsersConfigMap.Namespace,
		},
		Data: map[string]string{
			"a": "b",
		},
	}
	envSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.EnvSecret.Name,
			Namespace: config.EnvSecret.Namespace,
		},
		Data: map[string][]byte{
			"a": []byte("b"),
		},
	}
	certSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.OutputTLSConfigSecret.Name,
			Namespace: config.OutputTLSConfigSecret.Namespace,
		},
		Data: map[string][]byte{
			"a": []byte("b"),
		},
	}

	client := fake.NewClientBuilder().WithObjects(&dsConfig, &sectionsConfig, &filesConfig, &luaConfig, &parsersConfig, &envSecret, &certSecret).Build()
	r := Reconciler{
		Client: client,
		config: config,
	}
	ctx := context.Background()

	checksum, err := r.calculateChecksum(ctx)

	t.Run("Initial checksum should not be empty", func(t *testing.T) {
		require.NoError(t, err)
		require.NotEmpty(t, checksum)
	})

	t.Run("Changing static config should update checksum", func(t *testing.T) {
		dsConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &dsConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating static config")
		checksum = newChecksum
	})

	t.Run("Changing sections config should update checksum", func(t *testing.T) {
		sectionsConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &sectionsConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating sections config")
		checksum = newChecksum
	})

	t.Run("Changing files config should update checksum", func(t *testing.T) {
		filesConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &filesConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating files config")
		checksum = newChecksum
	})

	t.Run("Changing LUA config should update checksum", func(t *testing.T) {
		luaConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &luaConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating LUA config")
		checksum = newChecksum
	})

	t.Run("Changing parsers config should update checksum", func(t *testing.T) {
		parsersConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &parsersConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating parsers config")
		checksum = newChecksum
	})

	t.Run("Changing env Secret should update checksum", func(t *testing.T) {
		envSecret.Data["a"] = []byte("c")
		updateErr := client.Update(ctx, &envSecret)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating env secret")
		checksum = newChecksum
	})

	t.Run("Changing certificate Secret should update checksum", func(t *testing.T) {
		certSecret.Data["a"] = []byte("c")
		updateErr := client.Update(ctx, &certSecret)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating certificate secret")
	})
}
