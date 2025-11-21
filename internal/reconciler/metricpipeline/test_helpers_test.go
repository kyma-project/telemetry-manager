package metricpipeline

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
)

// reconcileResult holds the result of a reconciliation operation for test assertions.
type reconcileResult struct {
	result ctrl.Result
	err    error
}

// newTestClient creates a fake Kubernetes client for testing with the given objects.
// The client is configured with proper schemes, status subresources, and includes kube-system namespace.
func newTestClient(t *testing.T, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1alpha1.AddToScheme(scheme))

	kubeSystemNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
		},
	}

	allObjs := append([]client.Object{kubeSystemNamespace}, objs...)

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(allObjs...).WithStatusSubresource(objs...).Build()
}

// reconcileAndGet performs a reconciliation and returns the updated pipeline and any error.
// It's a helper to reduce boilerplate in tests.
func reconcileAndGet(t *testing.T, client client.Client, reconciler *Reconciler, pipelineName string) reconcileResult {
	result, err := reconciler.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: pipelineName},
	})

	return reconcileResult{result: result, err: err}
}

// Validator test constructor and options

// validatorOption is a functional option for configuring a test Validator.
type validatorOption func(*Validator)

// withEndpointValidator overrides the default endpoint validator.
func withEndpointValidator(validator EndpointValidator) validatorOption {
	return func(v *Validator) {
		v.EndpointValidator = validator
	}
}

// withTLSCertValidator overrides the default TLS certificate validator.
func withTLSCertValidator(validator TLSCertValidator) validatorOption {
	return func(v *Validator) {
		v.TLSCertValidator = validator
	}
}

// withSecretRefValidator overrides the default secret reference validator.
func withSecretRefValidator(validator SecretRefValidator) validatorOption {
	return func(v *Validator) {
		v.SecretRefValidator = validator
	}
}

// withPipelineLock overrides the default pipeline lock.
func withPipelineLock(lock PipelineLock) validatorOption {
	return func(v *Validator) {
		v.PipelineLock = lock
	}
}

// withTransformSpecValidator overrides the default transform spec validator.
func withTransformSpecValidator(validator TransformSpecValidator) validatorOption {
	return func(v *Validator) {
		v.TransformSpecValidator = validator
	}
}

// withFilterSpecValidator overrides the default filter spec validator.
func withFilterSpecValidator(validator FilterSpecValidator) validatorOption {
	return func(v *Validator) {
		v.FilterSpecValidator = validator
	}
}

// newTestValidator creates a Validator with all dependencies mocked by default.
// Use functional options to override specific dependencies.
// All validators pass by default, and the pipeline lock succeeds by default.
func newTestValidator(opts ...validatorOption) *Validator {
	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	validator := &Validator{
		EndpointValidator:      stubs.NewEndpointValidator(nil),
		TLSCertValidator:       stubs.NewTLSCertValidator(nil),
		SecretRefValidator:     stubs.NewSecretRefValidator(nil),
		PipelineLock:           pipelineLock,
		TransformSpecValidator: stubs.NewTransformSpecValidator(nil),
		FilterSpecValidator:    stubs.NewFilterSpecValidator(nil),
	}

	// Apply functional options to override defaults
	for _, opt := range opts {
		opt(validator)
	}

	return validator
}

// Reconciler test constructor

// newTestReconciler creates a Reconciler with all dependencies mocked by default.
// Use the production Option functions to override specific dependencies.
//
// Default behavior:
//   - Globals: Default target namespace and version
//   - AgentConfigBuilder: Returns empty config, no errors
//   - AgentApplierDeleter: All operations succeed
//   - AgentProber: Agent is ready
//   - GatewayConfigBuilder: Returns empty config, no errors
//   - GatewayApplierDeleter: All operations succeed
//   - GatewayProber: Gateway is ready
//   - GatewayFlowHealthProber: Flow is healthy
//   - AgentFlowHealthProber: Flow is healthy
//   - IstioStatusChecker: Istio is not active
//   - OverridesHandler: No overrides, not paused
//   - PipelineLock: Lock acquisition succeeds
//   - PipelineSyncer: Lock acquisition succeeds
//   - PipelineValidator: All validations pass
//   - ErrorToMessageConverter: Standard converter
func newTestReconciler(client client.Client, opts ...Option) *Reconciler {
	// Set up default mocks
	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, nil, nil)

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything).Return(nil)

	gatewayConfigBuilder := &mocks.GatewayConfigBuilder{}
	gatewayConfigBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, nil, nil)

	gatewayApplierDeleter := &mocks.GatewayApplierDeleter{}
	gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	gatewayFlowHealthProber := &mocks.GatewayFlowHealthProber{}
	gatewayFlowHealthProber.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelGatewayProbeResult{}, nil)

	agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
	agentFlowHealthProber.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelAgentProbeResult{}, nil)

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(&overrides.Config{}, nil)

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	pipelineSync := &mocks.PipelineSyncer{}
	pipelineSync.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)

	// Build default options with mocked dependencies
	defaultOpts := []Option{
		WithGlobals(config.NewGlobal(config.WithTargetNamespace("default"))),
		WithAgentConfigBuilder(agentConfigBuilder),
		WithAgentApplierDeleter(agentApplierDeleter),
		WithAgentProber(commonStatusStubs.NewDaemonSetProber(nil)),
		WithGatewayConfigBuilder(gatewayConfigBuilder),
		WithGatewayApplierDeleter(gatewayApplierDeleter),
		WithGatewayProber(commonStatusStubs.NewDeploymentSetProber(nil)),
		WithGatewayFlowHealthProber(gatewayFlowHealthProber),
		WithAgentFlowHealthProber(agentFlowHealthProber),
		WithIstioStatusChecker(&stubs.IstioStatusChecker{IsActive: false}),
		WithOverridesHandler(overridesHandler),
		WithPipelineLock(pipelineLock),
		WithPipelineSyncer(pipelineSync),
		WithPipelineValidator(newTestValidator()),
		WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),
	}

	// Merge default options with provided options (provided options will override defaults)
	allOpts := append(defaultOpts, opts...)

	return New(client, allOpts...)
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

func containsPipelines(pp []telemetryv1alpha1.MetricPipeline) any {
	return mock.MatchedBy(func(pipelines []telemetryv1alpha1.MetricPipeline) bool {
		if len(pipelines) != len(pp) {
			return false
		}

		pipelineMap := make(map[string]bool)
		for _, p := range pipelines {
			pipelineMap[p.Name] = true
		}

		for _, p := range pp {
			if !pipelineMap[p.Name] {
				return false
			}
		}

		return true
	})
}
