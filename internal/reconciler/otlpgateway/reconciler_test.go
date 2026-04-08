package otlpgateway

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpgateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/resources/coordinationconfig"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

type mocks struct {
	gatewayApplierDeleter *mockGatewayApplierDeleter
	configBuilder         *mockOTLPGatewayConfigBuilder
	istioStatusChecker    *mockIstioStatusChecker
}

type mockGatewayApplierDeleter struct {
	mock.Mock
}

func (m *mockGatewayApplierDeleter) ApplyResources(ctx context.Context, c client.Client, opts otelcollector.GatewayApplyOptions) error {
	args := m.Called(ctx, c, opts)
	return args.Error(0)
}

func (m *mockGatewayApplierDeleter) DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error {
	args := m.Called(ctx, c, isIstioActive)
	return args.Error(0)
}

type mockOTLPGatewayConfigBuilder struct {
	mock.Mock
}

func (m *mockOTLPGatewayConfigBuilder) Build(ctx context.Context, opts otlpgateway.BuildOptions) (*common.Config, common.EnvVars, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}

	return args.Get(0).(*common.Config), args.Get(1).(common.EnvVars), args.Error(2)
}

type mockIstioStatusChecker struct {
	mock.Mock
}

func (m *mockIstioStatusChecker) IsIstioActive(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func newTestClient(t *testing.T, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1beta1.AddToScheme(scheme))
	require.NoError(t, istiosecurityclientv1.AddToScheme(scheme))

	kymaSystemNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kyma-system",
		},
	}

	kubeSystemNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
			UID:  "test-cluster-id",
		},
	}

	allObjs := append([]client.Object{kymaSystemNamespace, kubeSystemNamespace}, objs...)

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(allObjs...).WithStatusSubresource(objs...).Build()
}

func newTestReconciler(fakeClient client.Client, mocks *mocks) *Reconciler {
	return &Reconciler{
		Client:                fakeClient,
		globals:               config.NewGlobal(config.WithTargetNamespace("kyma-system")),
		gatewayApplierDeleter: mocks.gatewayApplierDeleter,
		configBuilder:         mocks.configBuilder,
		istioStatusChecker:    mocks.istioStatusChecker,
	}
}

func newDefaultMocks() *mocks {
	return &mocks{
		gatewayApplierDeleter: &mockGatewayApplierDeleter{},
		configBuilder:         &mockOTLPGatewayConfigBuilder{},
		istioStatusChecker:    &mockIstioStatusChecker{},
	}
}

func newReconcileRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
	}
}

func TestReconcile_ConfigMapCreatedIfNotExists(t *testing.T) {
	ctx := context.Background()
	fakeClient := newTestClient(t)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false, nil)
	mocks.gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	var cm corev1.ConfigMap

	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      names.OTLPGatewayPipelinesSyncConfigMap,
		Namespace: "kyma-system",
	}, &cm)
	require.NoError(t, err)
	assert.Contains(t, cm.Data, coordinationconfig.ConfigMapDataKey)
}

func TestReconcile_NoPipelines_DeletesGateway(t *testing.T) {
	ctx := context.Background()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines: []",
		},
	}

	fakeClient := newTestClient(t, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false, nil)
	mocks.gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, false).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	mocks.gatewayApplierDeleter.AssertCalled(t, "DeleteResources", mock.Anything, mock.Anything, false)
}

func TestReconcile_SinglePipeline_DeploysGateway(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: test-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false, nil)
	mocks.configBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	mocks.configBuilder.AssertCalled(t, "Build", mock.Anything, mock.Anything, mock.Anything)
	mocks.gatewayApplierDeleter.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
}

func TestReconcile_GenerationMismatch_SkipsPipeline(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()
	pipeline.Generation = 2

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: test-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false, nil)
	mocks.gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	mocks.gatewayApplierDeleter.AssertCalled(t, "DeleteResources", mock.Anything, mock.Anything, mock.Anything)
	mocks.configBuilder.AssertNotCalled(t, "Build", mock.Anything, mock.Anything, mock.Anything)
}

func TestReconcile_PipelineDeleted_SkipsPipeline(t *testing.T) {
	ctx := context.Background()

	now := metav1.Now()
	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()
	pipeline.DeletionTimestamp = &now
	pipeline.Finalizers = []string{"test-finalizer"}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: test-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false, nil)
	mocks.gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	mocks.gatewayApplierDeleter.AssertCalled(t, "DeleteResources", mock.Anything, mock.Anything, mock.Anything)
}

func TestReconcile_MultiplePipelines_AggregatesConfig(t *testing.T) {
	ctx := context.Background()

	pipeline1 := testutils.NewTracePipelineBuilder().
		WithName("pipeline-1").
		Build()

	pipeline2 := testutils.NewTracePipelineBuilder().
		WithName("pipeline-2").
		Build()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: pipeline-1\n  generation: 1\n- name: pipeline-2\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline1, &pipeline2, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false, nil)
	mocks.configBuilder.On("Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.TracePipelines) == 2
	})).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	mocks.configBuilder.AssertCalled(t, "Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.TracePipelines) == 2
	}))
}

func TestReconcile_MissingPipeline_SkipsGracefully(t *testing.T) {
	ctx := context.Background()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: missing-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false, nil)
	mocks.gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	mocks.gatewayApplierDeleter.AssertCalled(t, "DeleteResources", mock.Anything, mock.Anything, mock.Anything)
}

func TestReconcile_IstioEnabled_PassesFlag(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: test-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(true, nil)
	mocks.configBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.MatchedBy(func(opts otelcollector.GatewayApplyOptions) bool {
		return opts.IstioEnabled == true
	})).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	mocks.gatewayApplierDeleter.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.MatchedBy(func(opts otelcollector.GatewayApplyOptions) bool {
		return opts.IstioEnabled == true
	}))
}

func TestFetchTracePipelines_NotFound(t *testing.T) {
	ctx := context.Background()
	fakeClient := newTestClient(t)
	mocks := newDefaultMocks()

	sut := newTestReconciler(fakeClient, mocks)

	refs := []coordinationconfig.PipelineReference{
		{Name: "missing-pipeline", Generation: 1},
	}

	pipelines, err := sut.fetchTracePipelines(ctx, refs)
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchTracePipelines_GenerationMismatch(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()
	pipeline.Generation = 5

	fakeClient := newTestClient(t, &pipeline)
	mocks := newDefaultMocks()

	sut := newTestReconciler(fakeClient, mocks)

	refs := []coordinationconfig.PipelineReference{
		{Name: "test-pipeline", Generation: 3},
	}

	pipelines, err := sut.fetchTracePipelines(ctx, refs)
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchTracePipelines_DeletionTimestamp(t *testing.T) {
	ctx := context.Background()

	now := metav1.Now()
	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()
	pipeline.DeletionTimestamp = &now
	pipeline.Finalizers = []string{"test-finalizer"}

	fakeClient := newTestClient(t, &pipeline)
	mocks := newDefaultMocks()

	sut := newTestReconciler(fakeClient, mocks)

	refs := []coordinationconfig.PipelineReference{
		{Name: "test-pipeline", Generation: 1},
	}

	pipelines, err := sut.fetchTracePipelines(ctx, refs)
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchTracePipelines_Success(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()
	pipeline.Generation = 1

	fakeClient := newTestClient(t, &pipeline)
	mocks := newDefaultMocks()

	sut := newTestReconciler(fakeClient, mocks)

	refs := []coordinationconfig.PipelineReference{
		{Name: "test-pipeline", Generation: 1},
	}

	pipelines, err := sut.fetchTracePipelines(ctx, refs)
	require.NoError(t, err)
	require.Len(t, pipelines, 1)
	assert.Equal(t, "test-pipeline", pipelines[0].Name)
}

func TestFetchTracePipelines_GetError(t *testing.T) {
	ctx := context.Background()

	fakeClient := newTestClient(t)
	mocks := newDefaultMocks()

	sut := newTestReconciler(fakeClient, mocks)

	sut.Client = &errorClient{err: assert.AnError}

	refs := []coordinationconfig.PipelineReference{
		{Name: "test-pipeline", Generation: 1},
	}

	_, err := sut.fetchTracePipelines(ctx, refs)
	require.Error(t, err)
}

type errorClient struct {
	client.Client

	err error
}

func (c *errorClient) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return c.err
}

func TestNewReconciler_WithOptions(t *testing.T) {
	fakeClient := newTestClient(t)
	globals := config.NewGlobal(config.WithTargetNamespace("test-namespace"))

	gad := &mockGatewayApplierDeleter{}
	cb := &mockOTLPGatewayConfigBuilder{}
	isc := &mockIstioStatusChecker{}
	oh := overrides.New(globals, fakeClient)

	reconciler := NewReconciler(
		fakeClient,
		WithGlobals(globals),
		WithGatewayApplierDeleter(gad),
		WithConfigBuilder(cb),
		WithIstioStatusChecker(isc),
		WithOverridesHandler(oh),
	)

	require.NotNil(t, reconciler)
	assert.Equal(t, fakeClient, reconciler.Client)
	assert.Equal(t, "test-namespace", reconciler.globals.TargetNamespace())
	assert.Equal(t, gad, reconciler.gatewayApplierDeleter)
	assert.Equal(t, cb, reconciler.configBuilder)
	assert.Equal(t, isc, reconciler.istioStatusChecker)
	assert.Equal(t, oh, reconciler.overridesHandler)
}

func TestGlobals(t *testing.T) {
	fakeClient := newTestClient(t)
	globals := config.NewGlobal(config.WithTargetNamespace("test-namespace"))

	reconciler := NewReconciler(fakeClient, WithGlobals(globals))

	globalsPtr := reconciler.Globals()
	require.NotNil(t, globalsPtr)
	assert.Equal(t, "test-namespace", globalsPtr.TargetNamespace())
}

func TestReconcile_LogPipeline_DeploysGateway(t *testing.T) {
	ctx := context.Background()

	logPipeline := testutils.NewLogPipelineBuilder().
		WithName("test-log-pipeline").
		WithOTLPOutput().
		Build()
	logPipeline.Generation = 1

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "logPipelines:\n- name: test-log-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &logPipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false, nil)
	mocks.configBuilder.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	mocks.configBuilder.AssertCalled(t, "Build", mock.Anything, mock.Anything)
	mocks.gatewayApplierDeleter.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
}

func TestReconcile_TraceAndLogPipelines_DeploysUnifiedGateway(t *testing.T) {
	ctx := context.Background()

	tracePipeline := testutils.NewTracePipelineBuilder().
		WithName("test-trace-pipeline").
		Build()

	logPipeline := testutils.NewLogPipelineBuilder().
		WithName("test-log-pipeline").
		WithOTLPOutput().
		Build()
	logPipeline.Generation = 1

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: test-trace-pipeline\n  generation: 1\nlogPipelines:\n- name: test-log-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &tracePipeline, &logPipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false, nil)
	mocks.configBuilder.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	// Verify config was built with both pipelines
	mocks.configBuilder.AssertCalled(t, "Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.TracePipelines) == 1 && len(opts.LogPipelines) == 1
	}))
}

// Tests for log pipeline scenarios
func TestFetchLogPipelines_NotFound(t *testing.T) {
	ctx := context.Background()
	fakeClient := newTestClient(t)
	mocks := newDefaultMocks()
	sut := newTestReconciler(fakeClient, mocks)

	refs := []coordinationconfig.PipelineReference{
		{Name: "non-existent", Generation: 1},
	}

	pipelines, err := sut.fetchLogPipelines(ctx, refs)
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchLogPipelines_GenerationMismatch(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewLogPipelineBuilder().
		WithName("test-log").
		WithOTLPOutput().
		Build()

	fakeClient := newTestClient(t, &pipeline)
	mocks := newDefaultMocks()
	sut := newTestReconciler(fakeClient, mocks)

	refs := []coordinationconfig.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation + 1}, // Different generation
	}

	pipelines, err := sut.fetchLogPipelines(ctx, refs)
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchLogPipelines_DeletionTimestamp(t *testing.T) {
	ctx := context.Background()

	now := metav1.Now()
	pipeline := testutils.NewLogPipelineBuilder().
		WithName("test-log").
		WithOTLPOutput().
		Build()
	pipeline.DeletionTimestamp = &now
	pipeline.Finalizers = []string{"test-finalizer"} // Need finalizer for fake client

	fakeClient := newTestClient(t, &pipeline)
	mocks := newDefaultMocks()
	sut := newTestReconciler(fakeClient, mocks)

	refs := []coordinationconfig.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation},
	}

	pipelines, err := sut.fetchLogPipelines(ctx, refs)
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchLogPipelines_Success(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewLogPipelineBuilder().
		WithName("test-log").
		WithOTLPOutput().
		Build()

	fakeClient := newTestClient(t, &pipeline)
	mocks := newDefaultMocks()
	sut := newTestReconciler(fakeClient, mocks)

	refs := []coordinationconfig.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation},
	}

	pipelines, err := sut.fetchLogPipelines(ctx, refs)
	require.NoError(t, err)
	require.Len(t, pipelines, 1)
	assert.Equal(t, pipeline.Name, pipelines[0].Name)
}

func TestFetchLogPipelines_GetError(t *testing.T) {
	ctx := context.Background()
	fakeClient := newTestClient(t)
	mocks := newDefaultMocks()
	sut := newTestReconciler(fakeClient, mocks)

	sut.Client = &errorClient{err: assert.AnError}

	refs := []coordinationconfig.PipelineReference{
		{Name: "test-log", Generation: 1},
	}

	_, err := sut.fetchLogPipelines(ctx, refs)
	require.Error(t, err)
}

func TestReconcile_OnlyLogPipelines_DeploysGateway(t *testing.T) {
	ctx := context.Background()

	logPipeline := testutils.NewLogPipelineBuilder().
		WithName("test-log").
		WithOTLPOutput().
		Build()
	logPipeline.Generation = 1

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "logPipelines:\n- name: test-log\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &logPipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false, nil)
	mocks.configBuilder.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	// Verify config was built with only log pipelines
	mocks.configBuilder.AssertCalled(t, "Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.TracePipelines) == 0 && len(opts.LogPipelines) == 1
	}))

	// Verify gateway resources were applied
	mocks.gatewayApplierDeleter.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
}

// TestOverrideFunctionality verifies that OTLP Gateway respects override configuration
func TestOverrideFunctionality(t *testing.T) {
	tests := []struct {
		name                  string
		paused                bool
		overrideError         error
		expectReconcile       bool
		expectResourcesDeploy bool
		expectResourcesDelete bool
	}{
		{
			name:                  "OTLP Gateway not paused - resources deployed",
			paused:                false,
			expectReconcile:       true,
			expectResourcesDeploy: true,
			expectResourcesDelete: false,
		},
		{
			name:                  "OTLP Gateway paused - resources deleted",
			paused:                true,
			expectReconcile:       false,
			expectResourcesDeploy: false,
			expectResourcesDelete: false,
		},
		{
			name:                  "Override handler returns error",
			overrideError:         assert.AnError,
			expectReconcile:       false,
			expectResourcesDeploy: false,
			expectResourcesDelete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			pipeline := testutils.NewTracePipelineBuilder().
				WithName("test-pipeline").
				Build()

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.OTLPGatewayPipelinesSyncConfigMap,
					Namespace: "kyma-system",
				},
				Data: map[string]string{
					coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: test-pipeline\n  generation: 1",
				},
			}

			fakeClient := newTestClient(t, &pipeline, cm)
			mocks := newDefaultMocks()

			mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false, nil)

			if tt.expectReconcile {
				mocks.configBuilder.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
				mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			}

			sut := newTestReconciler(fakeClient, mocks)

			// Create override handler with test client that returns specific override config
			switch {
			case tt.overrideError != nil:
				sut.overridesHandler = overrides.New(sut.globals, &overrideConfigErrorClient{err: tt.overrideError})
			case tt.paused:
				sut.overridesHandler = overrides.New(sut.globals, newOverrideConfigClient(t, true))
			default:
				sut.overridesHandler = overrides.New(sut.globals, newOverrideConfigClient(t, false))
			}

			_, err := sut.Reconcile(ctx, newReconcileRequest())

			if tt.overrideError != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.expectResourcesDeploy {
				mocks.gatewayApplierDeleter.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
			} else {
				mocks.gatewayApplierDeleter.AssertNotCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}

// newOverrideConfigClient creates a fake client that returns override ConfigMap with specified pause state
func newOverrideConfigClient(t *testing.T, paused bool) client.Client {
	pausedStr := "false"
	if paused {
		pausedStr = "true"
	}

	overrideConfig := fmt.Sprintf(`otlpGateway:
  paused: %s`, pausedStr)

	overridesCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OverrideConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			"overrides": overrideConfig,
		},
	}

	return newTestClient(t, overridesCM)
}

// overrideConfigErrorClient is a client that returns errors when fetching override ConfigMap
type overrideConfigErrorClient struct {
	client.Client

	err error
}

func (c *overrideConfigErrorClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if cm, ok := obj.(*corev1.ConfigMap); ok && key.Name == names.OverrideConfigMap {
		_ = cm
		return c.err
	}

	return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
}

// Tests for metric pipeline fetch scenarios
func TestFetchMetricPipelines_NotFound(t *testing.T) {
	ctx := context.Background()
	fakeClient := newTestClient(t)
	mocks := newDefaultMocks()
	sut := newTestReconciler(fakeClient, mocks)

	refs := []coordinationconfig.PipelineReference{
		{Name: "non-existent", Generation: 1},
	}

	pipelines, err := sut.fetchMetricPipelines(ctx, refs)
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchMetricPipelines_GenerationMismatch(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName("test-metric").
		Build()

	fakeClient := newTestClient(t, &pipeline)
	mocks := newDefaultMocks()
	sut := newTestReconciler(fakeClient, mocks)

	refs := []coordinationconfig.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation + 1},
	}

	pipelines, err := sut.fetchMetricPipelines(ctx, refs)
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchMetricPipelines_DeletionTimestamp(t *testing.T) {
	ctx := context.Background()

	now := metav1.Now()
	pipeline := testutils.NewMetricPipelineBuilder().
		WithName("test-metric").
		Build()
	pipeline.DeletionTimestamp = &now
	pipeline.Finalizers = []string{"test-finalizer"}

	fakeClient := newTestClient(t, &pipeline)
	mocks := newDefaultMocks()
	sut := newTestReconciler(fakeClient, mocks)

	refs := []coordinationconfig.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation},
	}

	pipelines, err := sut.fetchMetricPipelines(ctx, refs)
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchMetricPipelines_Success(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName("test-metric").
		Build()

	fakeClient := newTestClient(t, &pipeline)
	mocks := newDefaultMocks()
	sut := newTestReconciler(fakeClient, mocks)

	refs := []coordinationconfig.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation},
	}

	pipelines, err := sut.fetchMetricPipelines(ctx, refs)
	require.NoError(t, err)
	require.Len(t, pipelines, 1)
	assert.Equal(t, pipeline.Name, pipelines[0].Name)
}

func TestFetchMetricPipelines_GetError(t *testing.T) {
	ctx := context.Background()

	fakeClient := newTestClient(t)
	mocks := newDefaultMocks()
	sut := newTestReconciler(fakeClient, mocks)

	sut.Client = &errorClient{err: assert.AnError}

	refs := []coordinationconfig.PipelineReference{
		{Name: "test-metric", Generation: 1},
	}

	_, err := sut.fetchMetricPipelines(ctx, refs)
	require.Error(t, err)
}

func TestReconcile_MetricPipeline_DeploysGateway(t *testing.T) {
	ctx := context.Background()

	metricPipeline := testutils.NewMetricPipelineBuilder().
		WithName("test-metric-pipeline").
		Build()
	metricPipeline.Generation = 1

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "metricPipelines:\n- name: test-metric-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &metricPipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false, nil)
	mocks.configBuilder.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	mocks.configBuilder.AssertCalled(t, "Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.MetricPipelines) == 1
	}))
	mocks.gatewayApplierDeleter.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
}
