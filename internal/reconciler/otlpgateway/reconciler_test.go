package otlpgateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpgateway"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

type mocks struct {
	gatewayApplierDeleter *mockGatewayApplierDeleter
	configBuilder         *mockOTLPGatewayConfigBuilder
	gatewayProber         *mockProber
	istioStatusChecker    *mockIstioStatusChecker
	errToMsgConverter     *mockErrorToMessageConverter
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

type mockProber struct {
	mock.Mock
}

func (m *mockProber) IsReady(ctx context.Context, name types.NamespacedName) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

type mockIstioStatusChecker struct {
	mock.Mock
}

func (m *mockIstioStatusChecker) IsIstioActive(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

type mockErrorToMessageConverter struct {
	mock.Mock
}

func (m *mockErrorToMessageConverter) Convert(err error) string {
	args := m.Called(err)
	return args.String(0)
}

func newTestClient(t *testing.T, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1beta1.AddToScheme(scheme))

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
		gatewayProber:         mocks.gatewayProber,
		istioStatusChecker:    mocks.istioStatusChecker,
		errToMsgConverter:     mocks.errToMsgConverter,
	}
}

func newDefaultMocks() *mocks {
	return &mocks{
		gatewayApplierDeleter: &mockGatewayApplierDeleter{},
		configBuilder:         &mockOTLPGatewayConfigBuilder{},
		gatewayProber:         &mockProber{},
		istioStatusChecker:    &mockIstioStatusChecker{},
		errToMsgConverter:     &mockErrorToMessageConverter{},
	}
}

func TestReconcile_ConfigMapCreatedIfNotExists(t *testing.T) {
	ctx := context.Background()
	fakeClient := newTestClient(t)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false)
	mocks.gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)

	var cm corev1.ConfigMap

	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      otelcollector.OTLPGatewayConfigMapName,
		Namespace: "kyma-system",
	}, &cm)
	require.NoError(t, err)
	assert.Contains(t, cm.Data, otelcollector.ConfigMapDataKey)
}

func TestReconcile_NoPipelines_DeletesGateway(t *testing.T) {
	ctx := context.Background()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      otelcollector.OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			otelcollector.ConfigMapDataKey: "TracePipeline: []",
		},
	}

	fakeClient := newTestClient(t, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false)
	mocks.gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, false).Return(nil)

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, ctrl.Request{})
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
			Name:      otelcollector.OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			otelcollector.ConfigMapDataKey: "TracePipeline:\n- name: test-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false)
	mocks.configBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mocks.gatewayProber.On("IsReady", mock.Anything, mock.Anything).Return(nil)
	mocks.errToMsgConverter.On("Convert", mock.Anything).Return("")

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, ctrl.Request{})
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
			Name:      otelcollector.OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			otelcollector.ConfigMapDataKey: "TracePipeline:\n- name: test-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false)
	mocks.gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mocks.gatewayProber.On("IsReady", mock.Anything, mock.Anything).Return(nil)
	mocks.errToMsgConverter.On("Convert", mock.Anything).Return("")

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, ctrl.Request{})
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
			Name:      otelcollector.OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			otelcollector.ConfigMapDataKey: "TracePipeline:\n- name: test-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false)
	mocks.gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mocks.gatewayProber.On("IsReady", mock.Anything, mock.Anything).Return(nil)
	mocks.errToMsgConverter.On("Convert", mock.Anything).Return("")

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, ctrl.Request{})
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
			Name:      otelcollector.OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			otelcollector.ConfigMapDataKey: "TracePipeline:\n- name: pipeline-1\n  generation: 1\n- name: pipeline-2\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline1, &pipeline2, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false)
	mocks.configBuilder.On("Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.TracePipelines) == 2
	})).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mocks.gatewayProber.On("IsReady", mock.Anything, mock.Anything).Return(nil)
	mocks.errToMsgConverter.On("Convert", mock.Anything).Return("")

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)

	mocks.configBuilder.AssertCalled(t, "Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.TracePipelines) == 2
	}))
}

func TestReconcile_MissingPipeline_SkipsGracefully(t *testing.T) {
	ctx := context.Background()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      otelcollector.OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			otelcollector.ConfigMapDataKey: "TracePipeline:\n- name: missing-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false)
	mocks.gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mocks.gatewayProber.On("IsReady", mock.Anything, mock.Anything).Return(nil)
	mocks.errToMsgConverter.On("Convert", mock.Anything).Return("")

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, ctrl.Request{})
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
			Name:      otelcollector.OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			otelcollector.ConfigMapDataKey: "TracePipeline:\n- name: test-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(true)
	mocks.configBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.MatchedBy(func(opts otelcollector.GatewayApplyOptions) bool {
		return opts.IstioEnabled == true
	})).Return(nil)
	mocks.gatewayProber.On("IsReady", mock.Anything, mock.Anything).Return(nil)
	mocks.errToMsgConverter.On("Convert", mock.Anything).Return("")

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, ctrl.Request{})
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

	refs := []otelcollector.PipelineReference{
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

	refs := []otelcollector.PipelineReference{
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

	refs := []otelcollector.PipelineReference{
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

	refs := []otelcollector.PipelineReference{
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

	refs := []otelcollector.PipelineReference{
		{Name: "test-pipeline", Generation: 1},
	}

	_, err := sut.fetchTracePipelines(ctx, refs)
	require.Error(t, err)
}

type errorClient struct {
	client.Client

	err error
}

func (c *errorClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if c.err != nil {
		if _, ok := obj.(*telemetryv1beta1.TracePipeline); ok {
			return c.err
		}
	}

	return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
}

func TestNewReconciler_WithOptions(t *testing.T) {
	fakeClient := newTestClient(t)
	globals := config.NewGlobal(config.WithTargetNamespace("test-namespace"))

	gad := &mockGatewayApplierDeleter{}
	cb := &mockOTLPGatewayConfigBuilder{}
	gp := &mockProber{}
	isc := &mockIstioStatusChecker{}
	etmc := &mockErrorToMessageConverter{}

	reconciler := NewReconciler(
		fakeClient,
		WithGlobals(globals),
		WithGatewayApplierDeleter(gad),
		WithConfigBuilder(cb),
		WithGatewayProber(gp),
		WithIstioStatusChecker(isc),
		WithErrorToMessageConverter(etmc),
	)

	require.NotNil(t, reconciler)
	assert.Equal(t, fakeClient, reconciler.Client)
	assert.Equal(t, "test-namespace", reconciler.globals.TargetNamespace())
	assert.Equal(t, gad, reconciler.gatewayApplierDeleter)
	assert.Equal(t, cb, reconciler.configBuilder)
	assert.Equal(t, gp, reconciler.gatewayProber)
	assert.Equal(t, isc, reconciler.istioStatusChecker)
	assert.Equal(t, etmc, reconciler.errToMsgConverter)
}

func TestGlobals(t *testing.T) {
	fakeClient := newTestClient(t)
	globals := config.NewGlobal(config.WithTargetNamespace("test-namespace"))

	reconciler := NewReconciler(fakeClient, WithGlobals(globals))

	globalsPtr := reconciler.Globals()
	require.NotNil(t, globalsPtr)
	assert.Equal(t, "test-namespace", globalsPtr.TargetNamespace())
}

func TestUpdateTracePipelineCondition_NotFound(t *testing.T) {
	ctx := context.Background()
	fakeClient := newTestClient(t)
	mocks := newDefaultMocks()

	sut := newTestReconciler(fakeClient, mocks)

	condition := &metav1.Condition{
		Type:   "GatewayHealthy",
		Status: metav1.ConditionTrue,
		Reason: "GatewayReady",
	}

	err := sut.updateTracePipelineCondition(ctx, "non-existent-pipeline", condition)
	require.NoError(t, err) // Should not error for not found
}

func TestUpdateTracePipelineCondition_Success(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()
	pipeline.Generation = 5

	fakeClient := newTestClient(t, &pipeline)
	mocks := newDefaultMocks()

	sut := newTestReconciler(fakeClient, mocks)

	condition := &metav1.Condition{
		Type:   "GatewayHealthy",
		Status: metav1.ConditionTrue,
		Reason: "GatewayReady",
	}

	err := sut.updateTracePipelineCondition(ctx, "test-pipeline", condition)
	require.NoError(t, err)

	// Verify the condition was set
	var updatedPipeline telemetryv1beta1.TracePipeline

	err = fakeClient.Get(ctx, types.NamespacedName{Name: "test-pipeline"}, &updatedPipeline)
	require.NoError(t, err)

	cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, "GatewayHealthy")
	require.NotNil(t, cond)
	assert.Equal(t, metav1.ConditionTrue, cond.Status)
	assert.Equal(t, "GatewayReady", cond.Reason)
	assert.Equal(t, int64(5), cond.ObservedGeneration)
}

func TestUpdateLogPipelineCondition_Success(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewLogPipelineBuilder().
		WithName("test-log").
		WithOTLPOutput().
		Build()

	fakeClient := newTestClient(t, &pipeline)
	mocks := newDefaultMocks()

	sut := newTestReconciler(fakeClient, mocks)

	condition := &metav1.Condition{
		Type:   conditions.TypeGatewayHealthy,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonGatewayReady,
	}

	err := sut.updateLogPipelineCondition(ctx, pipeline.Name, condition)
	require.NoError(t, err)

	// Verify condition was set
	var updatedPipeline telemetryv1beta1.LogPipeline

	err = fakeClient.Get(ctx, types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
	require.NoError(t, err)

	cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeGatewayHealthy)
	require.NotNil(t, cond)
	assert.Equal(t, metav1.ConditionTrue, cond.Status)
	assert.Equal(t, conditions.ReasonGatewayReady, cond.Reason)
	assert.Equal(t, updatedPipeline.Generation, cond.ObservedGeneration)
}

func TestUpdateLogPipelineCondition_PipelineNotFound(t *testing.T) {
	ctx := context.Background()

	fakeClient := newTestClient(t)
	mocks := newDefaultMocks()

	sut := newTestReconciler(fakeClient, mocks)

	condition := &metav1.Condition{
		Type:   conditions.TypeGatewayHealthy,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonGatewayReady,
	}

	// Should not error when pipeline doesn't exist
	err := sut.updateLogPipelineCondition(ctx, "non-existent", condition)
	require.NoError(t, err)
}

func TestUpdateGatewayHealthyConditions_EmptyList(t *testing.T) {
	ctx := context.Background()
	fakeClient := newTestClient(t)
	mocks := newDefaultMocks()

	sut := newTestReconciler(fakeClient, mocks)

	err := sut.updateGatewayHealthyConditions(ctx, []string{})
	require.NoError(t, err)
}

func TestUpdateGatewayHealthyConditions_MultiplePipelines(t *testing.T) {
	ctx := context.Background()

	pipeline1 := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline-1").
		Build()

	pipeline2 := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline-2").
		Build()

	fakeClient := newTestClient(t, &pipeline1, &pipeline2)
	mocks := newDefaultMocks()

	mocks.gatewayProber.On("IsReady", mock.Anything, mock.Anything).Return(nil)
	mocks.errToMsgConverter.On("Convert", mock.Anything).Return("")

	sut := newTestReconciler(fakeClient, mocks)

	err := sut.updateGatewayHealthyConditions(ctx, []string{"test-pipeline-1", "test-pipeline-2"})
	require.NoError(t, err)

	// Verify both pipelines were updated
	var p1 telemetryv1beta1.TracePipeline

	err = fakeClient.Get(ctx, types.NamespacedName{Name: "test-pipeline-1"}, &p1)
	require.NoError(t, err)
	assert.NotEmpty(t, p1.Status.Conditions)

	var p2 telemetryv1beta1.TracePipeline

	err = fakeClient.Get(ctx, types.NamespacedName{Name: "test-pipeline-2"}, &p2)
	require.NoError(t, err)
	assert.NotEmpty(t, p2.Status.Conditions)
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
			Name:      otelcollector.OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			otelcollector.ConfigMapDataKey: "LogPipeline:\n- name: test-log-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &logPipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false)
	mocks.configBuilder.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mocks.gatewayProber.On("IsReady", mock.Anything, mock.Anything).Return(nil)
	mocks.errToMsgConverter.On("Convert", mock.Anything).Return("")

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, ctrl.Request{})
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
			Name:      otelcollector.OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			otelcollector.ConfigMapDataKey: "TracePipeline:\n- name: test-trace-pipeline\n  generation: 1\nLogPipeline:\n- name: test-log-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &tracePipeline, &logPipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false)
	mocks.configBuilder.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mocks.gatewayProber.On("IsReady", mock.Anything, mock.Anything).Return(nil)
	mocks.errToMsgConverter.On("Convert", mock.Anything).Return("")

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, ctrl.Request{})
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

	refs := []otelcollector.PipelineReference{
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

	refs := []otelcollector.PipelineReference{
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

	refs := []otelcollector.PipelineReference{
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

	refs := []otelcollector.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation},
	}

	pipelines, err := sut.fetchLogPipelines(ctx, refs)
	require.NoError(t, err)
	require.Len(t, pipelines, 1)
	assert.Equal(t, pipeline.Name, pipelines[0].Name)
}

func TestUpdateLogPipelineCondition_PipelineBeingDeleted(t *testing.T) {
	ctx := context.Background()

	now := metav1.Now()
	pipeline := testutils.NewLogPipelineBuilder().
		WithName("test-log").
		WithOTLPOutput().
		Build()
	pipeline.DeletionTimestamp = &now
	pipeline.Finalizers = []string{"test-finalizer"}

	fakeClient := newTestClient(t, &pipeline)
	mocks := newDefaultMocks()
	sut := newTestReconciler(fakeClient, mocks)

	condition := &metav1.Condition{
		Type:   conditions.TypeGatewayHealthy,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonGatewayReady,
	}

	// Should not error when pipeline is being deleted
	err := sut.updateLogPipelineCondition(ctx, pipeline.Name, condition)
	require.NoError(t, err)
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
			Name:      otelcollector.OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			otelcollector.ConfigMapDataKey: "LogPipeline:\n- name: test-log\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &logPipeline, cm)
	mocks := newDefaultMocks()

	mocks.istioStatusChecker.On("IsIstioActive", mock.Anything).Return(false)
	mocks.configBuilder.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
	mocks.gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mocks.gatewayProber.On("IsReady", mock.Anything, mock.Anything).Return(nil)
	mocks.errToMsgConverter.On("Convert", mock.Anything).Return("")

	sut := newTestReconciler(fakeClient, mocks)

	_, err := sut.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)

	// Verify config was built with only log pipelines
	mocks.configBuilder.AssertCalled(t, "Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.TracePipelines) == 0 && len(opts.LogPipelines) == 1
	}))

	// Verify gateway resources were applied
	mocks.gatewayApplierDeleter.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
}

func TestCollectAllReferencedNames(t *testing.T) {
	config := &otelcollector.OTLPGatewayConfigMap{
		TracePipeline: []otelcollector.PipelineReference{
			{Name: "trace1", Generation: 1},
			{Name: "trace2", Generation: 2},
		},
		LogPipeline: []otelcollector.PipelineReference{
			{Name: "log1", Generation: 1},
			{Name: "log2", Generation: 2},
		},
	}

	names := collectAllReferencedNames(config)

	require.Len(t, names, 4)
	assert.Contains(t, names, "trace1")
	assert.Contains(t, names, "trace2")
	assert.Contains(t, names, "log1")
	assert.Contains(t, names, "log2")
}

func TestCollectAllReferencedNames_Empty(t *testing.T) {
	config := &otelcollector.OTLPGatewayConfigMap{
		TracePipeline: []otelcollector.PipelineReference{},
		LogPipeline:   []otelcollector.PipelineReference{},
	}

	names := collectAllReferencedNames(config)

	require.Empty(t, names)
}
