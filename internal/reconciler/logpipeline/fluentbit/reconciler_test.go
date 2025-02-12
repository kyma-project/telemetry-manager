package fluentbit

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func TestReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	overridesHandlerStub := &logpipelinemocks.OverridesHandler{}
	overridesHandlerStub.On("LoadOverrides", context.Background()).Return(&overrides.Config{}, nil)

	istioStatusCheckerStub := &stubs.IstioStatusChecker{IsActive: false}

	testConfig := Config{
		DaemonSet:           types.NamespacedName{Name: "test-telemetry-fluent-bit", Namespace: "default"},
		SectionsConfigMap:   types.NamespacedName{Name: "test-telemetry-fluent-bit-sections", Namespace: "default"},
		FilesConfigMap:      types.NamespacedName{Name: "test-telemetry-fluent-bit-files", Namespace: "default"},
		LuaConfigMap:        types.NamespacedName{Name: "test-telemetry-fluent-bit-lua", Namespace: "default"},
		ParsersConfigMap:    types.NamespacedName{Name: "test-telemetry-fluent-bit-parsers", Namespace: "default"},
		EnvConfigSecret:     types.NamespacedName{Name: "test-telemetry-fluent-bit-env", Namespace: "default"},
		TLSFileConfigSecret: types.NamespacedName{Name: "test-telemetry-fluent-bit-output-tls-config", Namespace: "default"},
		DaemonSetConfig: fluentbit.DaemonSetConfig{
			FluentBitImage: "fluent/bit:dummy",
			ExporterImage:  "exporter:dummy",
		},
	}

	t.Run("should set status UnsupportedMode true if contains custom plugin", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").WithCustomFilter("Name grep").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		require.True(t, *updatedPipeline.Status.UnsupportedMode)
	})

	t.Run("should set status UnsupportedMode false if does not contains custom plugin", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		require.False(t, *updatedPipeline.Status.UnsupportedMode)
	})

	t.Run("no resources generated if app input disabled", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithApplicationInputDisabled().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		// check Fluent Bit sections configmap as an indicator of resources generation
		cm := &corev1.ConfigMap{}
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, cm)
		require.True(t, apierrors.IsNotFound(err), "sections configmap should not exist")
	})

	t.Run("log agent is not ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := commonStatusStubs.NewDaemonSetProber(workloadstatus.ErrDaemonSetNotFound)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}
		errToMsgStub.On("Convert", mock.Anything).Return("DaemonSet is not yet created")

		sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionFalse,
			conditions.ReasonAgentNotReady,
			workloadstatus.ErrDaemonSetNotFound.Error(),
		)

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.NoError(t, err, "sections configmap must exist")
		require.Contains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must contain pipeline name")
	})

	t.Run("log agent is ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionTrue,
			conditions.ReasonAgentReady,
			"Log agent DaemonSet is ready",
		)

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.NoError(t, err, "sections configmap must exist")
		require.Contains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must contain pipeline name")
	})

	t.Run("log agent prober fails", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := commonStatusStubs.NewDaemonSetProber(workloadstatus.ErrDaemonSetFetching)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &conditions.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionFalse,
			conditions.ReasonAgentNotReady,
			"Failed to get DaemonSet",
		)

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.NoError(t, err, "sections configmap must exist")
		require.Contains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must contain pipeline name")
	})

	t.Run("referenced secret missing", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().
			WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
			Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound)),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}
		errToMsgStub.On("Convert", mock.Anything).Return("")

		sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
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
			"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
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

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}
		errToMsgStub.On("Convert", mock.Anything).Return("")

		sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionTrue,
			conditions.ReasonAgentConfigured,
			"LogPipeline specification is successfully applied to the configuration of Log agent",
		)

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.NoError(t, err, "sections configmap must exist")
		require.Contains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must contain pipeline name")
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

				proberStub := commonStatusStubs.NewDaemonSetProber(nil)

				flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

				pipelineValidatorWithStubs := &Validator{
					EndpointValidator:  stubs.NewEndpointValidator(nil),
					TLSCertValidator:   stubs.NewTLSCertValidator(nil),
					SecretRefValidator: stubs.NewSecretRefValidator(nil),
				}

				errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}
				errToMsgStub.On("Convert", mock.Anything).Return("")

				sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

				var pl1 telemetryv1alpha1.LogPipeline

				require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
				err := sut.Reconcile(context.Background(), &pl1)
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.LogPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				requireHasStatusCondition(t, updatedPipeline,
					conditions.TypeFlowHealthy,
					tt.expectedStatus,
					tt.expectedReason,
					tt.expectedMessage,
				)

				var cm corev1.ConfigMap
				err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
				require.NoError(t, err, "sections configmap must exist")
				require.Contains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must contain pipeline name")
			})
		}
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
				name:            "ca expired",
				tlsCertErr:      &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSCertificateExpired,
				expectedMessage: "TLS CA certificate expired on 2020-11-01",
			},
			{
				name:                  "ca about to expire",
				tlsCertErr:            &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
				expectedStatus:        metav1.ConditionTrue,
				expectedReason:        conditions.ReasonTLSCertificateAboutToExpire,
				expectedMessage:       "TLS CA certificate is about to expire, configured certificate is valid until 2024-11-01",
				expectAgentConfigured: true,
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
				pipeline := testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					WithHTTPOutput(testutils.HTTPClientTLSFromString("ca", "fooCert", "fooKey")).
					Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				proberStub := commonStatusStubs.NewDaemonSetProber(nil)

				flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

				pipelineValidatorWithStubs := &Validator{
					EndpointValidator:  stubs.NewEndpointValidator(nil),
					TLSCertValidator:   stubs.NewTLSCertValidator(nil),
					SecretRefValidator: stubs.NewSecretRefValidator(tt.tlsCertErr),
				}

				errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}
				errToMsgStub.On("Convert", mock.Anything).Return("")

				sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

				var pl1 telemetryv1alpha1.LogPipeline

				require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
				err := sut.Reconcile(context.Background(), &pl1)
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
						"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
					)
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

	t.Run("Check different Pod Error Conditions", func(t *testing.T) {
		tests := []struct {
			name            string
			probeErr        error
			expectedStatus  metav1.ConditionStatus
			expectedReason  string
			expectedMessage string
		}{
			{
				name:            "pod is OOM",
				probeErr:        &workloadstatus.PodIsPendingError{ContainerName: "foo", Reason: "OOMKilled", Message: ""},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonAgentNotReady,
				expectedMessage: "Pod is in the pending state because container: foo is not running due to: OOMKilled. Please check the container: foo logs.",
			},
			{
				name:            "pod is CrashLoop",
				probeErr:        &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonAgentNotReady,
				expectedMessage: "Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
			},
			{
				name:            "no Pods deployed",
				probeErr:        workloadstatus.ErrNoPodsDeployed,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonAgentNotReady,
				expectedMessage: "No Pods deployed",
			},
			{
				name:            "fluent bit rollout in progress",
				probeErr:        &workloadstatus.RolloutInProgressError{},
				expectedStatus:  metav1.ConditionTrue,
				expectedReason:  conditions.ReasonRolloutInProgress,
				expectedMessage: "Pods are being started/updated",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				proberStub := commonStatusStubs.NewDaemonSetProber(tt.probeErr)

				flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

				pipelineValidatorWithStubs := &Validator{
					EndpointValidator:  stubs.NewEndpointValidator(nil),
					TLSCertValidator:   stubs.NewTLSCertValidator(nil),
					SecretRefValidator: stubs.NewSecretRefValidator(nil),
				}

				errToMsgStub := &conditions.ErrorToMessageConverter{}

				sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

				var pl1 telemetryv1alpha1.LogPipeline

				require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
				err := sut.Reconcile(context.Background(), &pl1)
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.LogPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
				cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeAgentHealthy)
				require.Equal(t, tt.expectedStatus, cond.Status)
				require.Equal(t, tt.expectedReason, cond.Reason)

				require.Equal(t, tt.expectedMessage, cond.Message)

				var cm corev1.ConfigMap
				err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
				require.NoError(t, err, "sections configmap must exist")
				require.Contains(t, cm.Data[pipeline.Name+".conf"], pipeline.Name, "sections configmap must contain pipeline name")
			})
		}
	})

	t.Run("a request to the Kubernetes API server has failed when validating the secret references", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().
			WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
			Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		serverErr := errors.New("failed to get secret: server error")
		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(&errortypes.APIRequestFailedError{Err: serverErr}),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.True(t, errors.Is(err, serverErr))

		var updatedPipeline telemetryv1alpha1.LogPipeline
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
			"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
		)

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, &cm)
		require.Error(t, err, "sections configmap should not exist")
	})

	t.Run("create 2 pipelines and delete 1 should update sections configmap properly", func(t *testing.T) {
		pipeline1 := testutils.NewLogPipelineBuilder().
			WithName("pipeline1").
			WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
			WithHTTPOutput(testutils.HTTPHost("host")).
			Build()
		pipeline2 := testutils.NewLogPipelineBuilder().
			WithName("pipeline2").
			WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
			WithHTTPOutput(testutils.HTTPHost("host")).
			Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline1, &pipeline2).WithStatusSubresource(&pipeline1, &pipeline2).Build()
		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline1.Name).Return(prober.LogPipelineProbeResult{}, nil)
		flowHealthProberStub.On("Probe", mock.Anything, pipeline2.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline1.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var pl2 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline2.Name}, &pl2))
		err = sut.Reconcile(context.Background(), &pl2)
		require.NoError(t, err)

		cm := &corev1.ConfigMap{}
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, cm)
		require.NoError(t, err, "sections configmap must exist")
		require.Contains(t, cm.Data[pipeline1.Name+".conf"], pipeline1.Name, "sections configmap must contain pipeline1 name")
		require.Contains(t, cm.Data[pipeline2.Name+".conf"], pipeline2.Name, "sections configmap must contain pipeline2 name")

		pipeline1Deleted := testutils.NewLogPipelineBuilder().
			WithName("pipeline1").
			WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
			WithHTTPOutput(testutils.HTTPHost("host")).
			WithDeletionTimeStamp(metav1.Now()).
			Build()

		fakeClient.Delete(context.Background(), &pipeline1)
		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline1.Name}, &pl1))
		err = sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		pipeline1 = pipeline1Deleted
		err = fakeClient.Get(context.Background(), testConfig.SectionsConfigMap, cm)
		require.NoError(t, err, "sections configmap must exist")
		require.NotContains(t, cm.Data[pipeline1.Name+".conf"], pipeline1.Name, "sections configmap must not contain pipeline1")
		require.Contains(t, cm.Data[pipeline2.Name+".conf"], pipeline2.Name, "sections configmap must contain pipeline2 name")
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
		EnvConfigSecret: types.NamespacedName{
			Namespace: "default",
			Name:      "env",
		},
		TLSFileConfigSecret: types.NamespacedName{
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
			Name:      config.EnvConfigSecret.Name,
			Namespace: config.EnvConfigSecret.Namespace,
		},
		Data: map[string][]byte{
			"a": []byte("b"),
		},
	}
	certSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.TLSFileConfigSecret.Name,
			Namespace: config.TLSFileConfigSecret.Namespace,
		},
		Data: map[string][]byte{
			"a": []byte("b"),
		},
	}

	client := fake.NewClientBuilder().WithObjects(&dsConfig, &sectionsConfig, &filesConfig, &luaConfig, &parsersConfig, &envSecret, &certSecret).Build()

	r := New(client, config, nil, nil, nil, nil, nil)
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
