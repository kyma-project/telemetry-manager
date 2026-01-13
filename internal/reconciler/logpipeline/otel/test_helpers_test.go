package otel

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/otel/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
)

// newTestClient creates a fake Kubernetes client with the telemetry scheme for testing.
func newTestClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1beta1.AddToScheme(scheme))

	// Create kube-system namespace required by reconciler for cluster UID
	kubeSystemNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
			UID:  "test-cluster-uid",
		},
	}

	allObjs := append([]client.Object{kubeSystemNs}, objs...)

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(allObjs...).
		WithStatusSubresource(objs...).
		Build()
}

// requireHasStatusCondition asserts that a LogPipeline has a specific status condition.
func requireHasStatusCondition(t *testing.T, pipeline telemetryv1beta1.LogPipeline, condType string, status metav1.ConditionStatus, reason, message string) {
	t.Helper()

	cond := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
	require.NotNil(t, cond, "could not find condition of type %s", condType)
	require.Equal(t, status, cond.Status)
	require.Equal(t, reason, cond.Reason)
	require.Equal(t, message, cond.Message)
	require.Equal(t, pipeline.Generation, cond.ObservedGeneration)
	require.NotEmpty(t, cond.LastTransitionTime)
}

func requireHasStatusConditionObject(t *testing.T, pipeline telemetryv1beta1.LogPipeline, expectedCond metav1.Condition) {
	t.Helper()

	requireHasStatusCondition(t, pipeline, expectedCond.Type, expectedCond.Status, expectedCond.Reason, expectedCond.Message)
}

// reconcileResult holds the result of a reconciliation operation for test assertions.
type reconcileResult struct {
	pipeline telemetryv1beta1.LogPipeline
	err      error
}

// reconcileAndGet performs a reconciliation and returns the updated pipeline and any error.
// It's a helper to reduce boilerplate in tests.
func reconcileAndGet(t *testing.T, client client.Client, reconciler *Reconciler, pipelineName string) reconcileResult {
	var pl telemetryv1beta1.LogPipeline
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{Name: pipelineName}, &pl))

	err := reconciler.Reconcile(t.Context(), &pl)

	var updatedPipeline telemetryv1beta1.LogPipeline
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{Name: pipelineName}, &updatedPipeline))

	return reconcileResult{pipeline: updatedPipeline, err: err}
}

// newTestValidator creates a Validator with all dependencies mocked by default.
// Dependencies return no errors (happy path) unless overridden via options.
func newTestValidator(opts ...ValidatorOption) *Validator {
	// Create mock pipeline lock that allows all operations by default
	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("ReleaseLock", mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	// Create validator with all validations passing by default
	allOpts := []ValidatorOption{
		WithValidatorPipelineLock(pipelineLock),
		WithEndpointValidator(stubs.NewEndpointValidator(nil)),
		WithTLSCertValidator(stubs.NewTLSCertValidator(nil)),
		WithSecretRefValidator(stubs.NewSecretRefValidator(nil)),
		WithTransformSpecValidator(stubs.NewTransformSpecValidator(nil)),
		WithFilterSpecValidator(stubs.NewFilterSpecValidator(nil)),
	}

	allOpts = append(allOpts, opts...)

	v := &Validator{}
	// Apply custom options to override defaults
	for _, opt := range allOpts {
		opt(v)
	}

	return v
}

// newTestReconciler creates a Reconciler with default mocked dependencies for testing.
// Uses production Option type (capitalized With* functions) to configure dependencies.
// Provided options override the defaults.
func newTestReconciler(client client.Client, opts ...Option) *Reconciler {
	// Set up default mocked dependencies - all operations succeed by default
	gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
	gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything, mock.Anything).
		Return(&common.Config{}, common.EnvVars{}, nil)

	agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
	agentConfigBuilderMock.On("Build", mock.Anything, mock.Anything, mock.Anything).
		Return(&common.Config{}, common.EnvVars{}, nil)

	gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
	gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	gatewayApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
	agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil)

	gatewayFlowHealthProberMock := &mocks.GatewayFlowHealthProber{}
	gatewayFlowHealthProberMock.On("Probe", mock.Anything, mock.Anything).
		Return(prober.OTelGatewayProbeResult{}, nil)

	agentFlowHealthProberMock := &mocks.AgentFlowHealthProber{}
	agentFlowHealthProberMock.On("Probe", mock.Anything, mock.Anything).
		Return(prober.OTelAgentProbeResult{}, nil)

	gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
	agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

	istioStatusCheckerStub := &stubs.IstioStatusChecker{IsActive: false}

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("ReleaseLock", mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	// Create validator with passing validations by default
	pipelineValidator := newTestValidator(WithValidatorPipelineLock(pipelineLock))

	errToMsg := &conditions.ErrorToMessageConverter{}

	// Build default options with all mocked dependencies
	allOpts := []Option{
		WithClient(client),
		WithGlobals(config.NewGlobal(config.WithTargetNamespace("default"), config.WithVersion("1.0.0"))),
		WithGatewayFlowHealthProber(gatewayFlowHealthProberMock),
		WithAgentFlowHealthProber(agentFlowHealthProberMock),
		WithAgentConfigBuilder(agentConfigBuilderMock),
		WithAgentApplierDeleter(agentApplierDeleterMock),
		WithAgentProber(agentProberStub),
		WithGatewayApplierDeleter(gatewayApplierDeleterMock),
		WithGatewayConfigBuilder(gatewayConfigBuilderMock),
		WithGatewayProber(gatewayProberStub),
		WithIstioStatusChecker(istioStatusCheckerStub),
		WithPipelineLock(pipelineLock),
		WithPipelineValidator(pipelineValidator),
		WithErrorToMessageConverter(errToMsg),
	}

	// Merge default options with provided options (provided options override defaults)
	allOpts = append(allOpts, opts...)

	return New(allOpts...)
}
