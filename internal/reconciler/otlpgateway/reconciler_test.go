package otlpgateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpgateway"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/otlpgateway/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/otlpgateway/stubs"
	"github.com/kyma-project/telemetry-manager/internal/resources/coordinationconfig"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

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

// newTestReconciler creates a Reconciler with default empty mocks. Pass With* options to override
// specific dependencies when you need to configure expectations or assert calls.
func newTestReconciler(fakeClient client.Client, opts ...Option) *Reconciler {
	r := &Reconciler{
		Client:                fakeClient,
		globals:               config.NewGlobal(config.WithTargetNamespace("kyma-system")),
		gatewayApplierDeleter: &mocks.GatewayApplierDeleter{},
		configBuilder:         &mocks.OTLPGatewayConfigBuilder{},
		istioStatusChecker:    &stubs.IstioStatusChecker{},
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

func newReconcileRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
	}
}

func TestReconcile_MissingConfigMap_DeletesGateway(t *testing.T) {
	ctx := context.Background()
	fakeClient := newTestClient(t) // no ConfigMap pre-created

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("DeleteResources", mock.Anything, mock.Anything, false, false).Return(nil)

	sut := newTestReconciler(fakeClient,
		WithGatewayApplierDeleter(gad),
		WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
	)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	gad.AssertCalled(t, "DeleteResources", mock.Anything, mock.Anything, false, false)

	var cm corev1.ConfigMap

	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      names.OTLPGatewayCoordinationConfigMap,
		Namespace: "kyma-system",
	}, &cm)
	require.True(t, apierrors.IsNotFound(err), "ConfigMap should not be created by the OTLP gateway reconciler")
}

func TestReconcile_NoPipelines_DeletesGateway(t *testing.T) {
	ctx := context.Background()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines: []",
		},
	}

	fakeClient := newTestClient(t, cm)

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("DeleteResources", mock.Anything, mock.Anything, false, false).Return(nil)

	sut := newTestReconciler(fakeClient,
		WithGatewayApplierDeleter(gad),
		WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
	)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	gad.AssertCalled(t, "DeleteResources", mock.Anything, mock.Anything, false, false)
}

func TestReconcile_SinglePipeline_DeploysGateway(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: test-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline, cm)

	cb := &mocks.OTLPGatewayConfigBuilder{}
	cb.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient,
		WithConfigBuilder(cb),
		WithGatewayApplierDeleter(gad),
		WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
		WithNodeSizeTracker(&stubs.NodeSizeTracker{MaxMemory: resource.Quantity{}}),
	)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	cb.AssertCalled(t, "Build", mock.Anything, mock.Anything, mock.Anything)
	gad.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
}

func TestReconcile_GenerationMismatch_SkipsPipeline(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()
	pipeline.Generation = 2

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: test-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline, cm)

	cb := &mocks.OTLPGatewayConfigBuilder{}

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("DeleteResources", mock.Anything, mock.Anything, false, false).Return(nil)

	sut := newTestReconciler(fakeClient,
		WithConfigBuilder(cb),
		WithGatewayApplierDeleter(gad),
		WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
	)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	gad.AssertCalled(t, "DeleteResources", mock.Anything, mock.Anything, false, false)
	cb.AssertNotCalled(t, "Build", mock.Anything, mock.Anything, mock.Anything)
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
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: test-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline, cm)

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("DeleteResources", mock.Anything, mock.Anything, false, false).Return(nil)

	sut := newTestReconciler(fakeClient,
		WithGatewayApplierDeleter(gad),
		WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
	)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	gad.AssertCalled(t, "DeleteResources", mock.Anything, mock.Anything, false, false)
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
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: pipeline-1\n  generation: 1\n- name: pipeline-2\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline1, &pipeline2, cm)

	cb := &mocks.OTLPGatewayConfigBuilder{}
	cb.On("Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.TracePipelines) == 2
	})).Return(&common.Config{}, common.EnvVars{}, nil)

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient,
		WithConfigBuilder(cb),
		WithGatewayApplierDeleter(gad),
		WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
		WithNodeSizeTracker(&stubs.NodeSizeTracker{MaxMemory: resource.Quantity{}}),
	)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	cb.AssertCalled(t, "Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.TracePipelines) == 2
	}))
}

func TestReconcile_MissingPipeline_SkipsGracefully(t *testing.T) {
	ctx := context.Background()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: missing-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, cm)

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("DeleteResources", mock.Anything, mock.Anything, false, false).Return(nil)

	sut := newTestReconciler(fakeClient,
		WithGatewayApplierDeleter(gad),
		WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
	)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	gad.AssertCalled(t, "DeleteResources", mock.Anything, mock.Anything, false, false)
}

func TestReconcile_IstioEnabled_PassesFlag(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: test-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &pipeline, cm)

	cb := &mocks.OTLPGatewayConfigBuilder{}
	cb.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("ApplyResources", mock.Anything, mock.Anything, mock.MatchedBy(func(opts otelcollector.GatewayApplyOptions) bool {
		return opts.IstioEnabled == true
	})).Return(nil)

	sut := newTestReconciler(fakeClient,
		WithConfigBuilder(cb),
		WithGatewayApplierDeleter(gad),
		WithIstioStatusChecker(&stubs.IstioStatusChecker{IsActive: true}),
		WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
		WithNodeSizeTracker(&stubs.NodeSizeTracker{MaxMemory: resource.Quantity{}}),
	)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	gad.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.MatchedBy(func(opts otelcollector.GatewayApplyOptions) bool {
		return opts.IstioEnabled == true
	}))
}

func TestFetchTracePipelines_NotFound(t *testing.T) {
	ctx := context.Background()

	sut := newTestReconciler(newTestClient(t))

	pipelines, err := sut.fetchTracePipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: "missing-pipeline", Generation: 1},
	})
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchTracePipelines_GenerationMismatch(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()
	pipeline.Generation = 5

	sut := newTestReconciler(newTestClient(t, &pipeline))

	pipelines, err := sut.fetchTracePipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: "test-pipeline", Generation: 3},
	})
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

	sut := newTestReconciler(newTestClient(t, &pipeline))

	pipelines, err := sut.fetchTracePipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: "test-pipeline", Generation: 1},
	})
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchTracePipelines_Success(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewTracePipelineBuilder().
		WithName("test-pipeline").
		Build()
	pipeline.Generation = 1

	sut := newTestReconciler(newTestClient(t, &pipeline))

	pipelines, err := sut.fetchTracePipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: "test-pipeline", Generation: 1},
	})
	require.NoError(t, err)
	require.Len(t, pipelines, 1)
	assert.Equal(t, "test-pipeline", pipelines[0].Name)
}

func TestFetchTracePipelines_GetError(t *testing.T) {
	ctx := context.Background()

	sut := newTestReconciler(newTestClient(t))
	sut.Client = &stubs.ErrorClient{Err: assert.AnError}

	_, err := sut.fetchTracePipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: "test-pipeline", Generation: 1},
	})
	require.Error(t, err)
}

func TestNewReconciler_WithOptions(t *testing.T) {
	fakeClient := newTestClient(t)
	globals := config.NewGlobal(config.WithTargetNamespace("test-namespace"))

	gad := &mocks.GatewayApplierDeleter{}
	cb := &mocks.OTLPGatewayConfigBuilder{}
	isc := &stubs.IstioStatusChecker{}
	oh := &stubs.OverridesHandler{}

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
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "logPipelines:\n- name: test-log-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &logPipeline, cm)

	cb := &mocks.OTLPGatewayConfigBuilder{}
	cb.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient,
		WithConfigBuilder(cb),
		WithGatewayApplierDeleter(gad),
		WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
		WithNodeSizeTracker(&stubs.NodeSizeTracker{MaxMemory: resource.Quantity{}}),
	)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	cb.AssertCalled(t, "Build", mock.Anything, mock.Anything)
	gad.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
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
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: test-trace-pipeline\n  generation: 1\nlogPipelines:\n- name: test-log-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &tracePipeline, &logPipeline, cm)

	cb := &mocks.OTLPGatewayConfigBuilder{}
	cb.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient,
		WithConfigBuilder(cb),
		WithGatewayApplierDeleter(gad),
		WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
		WithNodeSizeTracker(&stubs.NodeSizeTracker{MaxMemory: resource.Quantity{}}),
	)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	// Verify config was built with both pipelines
	cb.AssertCalled(t, "Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.TracePipelines) == 1 && len(opts.LogPipelines) == 1
	}))
}

// Tests for log pipeline scenarios
func TestFetchLogPipelines_NotFound(t *testing.T) {
	ctx := context.Background()

	sut := newTestReconciler(newTestClient(t))

	pipelines, err := sut.fetchLogPipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: "non-existent", Generation: 1},
	})
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchLogPipelines_GenerationMismatch(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewLogPipelineBuilder().
		WithName("test-log").
		WithOTLPOutput().
		Build()

	sut := newTestReconciler(newTestClient(t, &pipeline))

	pipelines, err := sut.fetchLogPipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation + 1},
	})
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

	sut := newTestReconciler(newTestClient(t, &pipeline))

	pipelines, err := sut.fetchLogPipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation},
	})
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchLogPipelines_Success(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewLogPipelineBuilder().
		WithName("test-log").
		WithOTLPOutput().
		Build()

	sut := newTestReconciler(newTestClient(t, &pipeline))

	pipelines, err := sut.fetchLogPipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation},
	})
	require.NoError(t, err)
	require.Len(t, pipelines, 1)
	assert.Equal(t, pipeline.Name, pipelines[0].Name)
}

func TestFetchLogPipelines_GetError(t *testing.T) {
	ctx := context.Background()

	sut := newTestReconciler(newTestClient(t))
	sut.Client = &stubs.ErrorClient{Err: assert.AnError}

	_, err := sut.fetchLogPipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: "test-log", Generation: 1},
	})
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
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "logPipelines:\n- name: test-log\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &logPipeline, cm)

	cb := &mocks.OTLPGatewayConfigBuilder{}
	cb.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient,
		WithConfigBuilder(cb),
		WithGatewayApplierDeleter(gad),
		WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
		WithNodeSizeTracker(&stubs.NodeSizeTracker{MaxMemory: resource.Quantity{}}),
	)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	// Verify config was built with only log pipelines
	cb.AssertCalled(t, "Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.TracePipelines) == 0 && len(opts.LogPipelines) == 1
	}))

	// Verify gateway resources were applied
	gad.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
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
					Name:      names.OTLPGatewayCoordinationConfigMap,
					Namespace: "kyma-system",
				},
				Data: map[string]string{
					coordinationconfig.ConfigMapDataKey: "tracePipelines:\n- name: test-pipeline\n  generation: 1",
				},
			}

			fakeClient := newTestClient(t, &pipeline, cm)

			gad := &mocks.GatewayApplierDeleter{}

			opts := []Option{
				WithGatewayApplierDeleter(gad),
				WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
				WithNodeSizeTracker(&stubs.NodeSizeTracker{MaxMemory: resource.Quantity{}}),
				WithOverridesHandler(&stubs.OverridesHandler{Paused: tt.paused, Err: tt.overrideError}),
			}

			if tt.expectReconcile {
				cb := &mocks.OTLPGatewayConfigBuilder{}
				cb.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)
				gad.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				opts = append(opts, WithConfigBuilder(cb))
			}

			sut := newTestReconciler(fakeClient, opts...)

			_, err := sut.Reconcile(ctx, newReconcileRequest())

			if tt.overrideError != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.expectResourcesDeploy {
				gad.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
			} else {
				gad.AssertNotCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}

// Tests for metric pipeline fetch scenarios
func TestFetchMetricPipelines_NotFound(t *testing.T) {
	ctx := context.Background()

	sut := newTestReconciler(newTestClient(t))

	pipelines, err := sut.fetchMetricPipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: "non-existent", Generation: 1},
	})
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchMetricPipelines_GenerationMismatch(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName("test-metric").
		Build()

	sut := newTestReconciler(newTestClient(t, &pipeline))

	pipelines, err := sut.fetchMetricPipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation + 1},
	})
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

	sut := newTestReconciler(newTestClient(t, &pipeline))

	pipelines, err := sut.fetchMetricPipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation},
	})
	require.NoError(t, err)
	assert.Empty(t, pipelines)
}

func TestFetchMetricPipelines_Success(t *testing.T) {
	ctx := context.Background()

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName("test-metric").
		Build()

	sut := newTestReconciler(newTestClient(t, &pipeline))

	pipelines, err := sut.fetchMetricPipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: pipeline.Name, Generation: pipeline.Generation},
	})
	require.NoError(t, err)
	require.Len(t, pipelines, 1)
	assert.Equal(t, pipeline.Name, pipelines[0].Name)
}

func TestFetchMetricPipelines_GetError(t *testing.T) {
	ctx := context.Background()

	sut := newTestReconciler(newTestClient(t))
	sut.Client = &stubs.ErrorClient{Err: assert.AnError}

	_, err := sut.fetchMetricPipelines(ctx, []coordinationconfig.PipelineReference{
		{Name: "test-metric", Generation: 1},
	})
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
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			coordinationconfig.ConfigMapDataKey: "metricPipelines:\n- name: test-metric-pipeline\n  generation: 1",
		},
	}

	fakeClient := newTestClient(t, &metricPipeline, cm)

	cb := &mocks.OTLPGatewayConfigBuilder{}
	cb.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil)

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sut := newTestReconciler(fakeClient,
		WithConfigBuilder(cb),
		WithGatewayApplierDeleter(gad),
		WithVpaStatusChecker(&stubs.VpaStatusChecker{CRDExists: false}),
		WithNodeSizeTracker(&stubs.NodeSizeTracker{MaxMemory: resource.Quantity{}}),
	)

	_, err := sut.Reconcile(ctx, newReconcileRequest())
	require.NoError(t, err)

	cb.AssertCalled(t, "Build", mock.Anything, mock.MatchedBy(func(opts otlpgateway.BuildOptions) bool {
		return len(opts.MetricPipelines) == 1
	}))
	gad.AssertCalled(t, "ApplyResources", mock.Anything, mock.Anything, mock.Anything)
}
