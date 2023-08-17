package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry/mocks"
)

func initReconciler(fakeClient client.Client) *Reconciler {
	scheme := runtime.NewScheme()

	config := Config{
		Traces: TracesConfig{
			OTLPServiceName: "trace-otlp-svc",
			Namespace:       "default",
		},
		Metrics: MetricsConfig{
			OTLPServiceName: "metric-otlp-svc",
			Namespace:       "default",
		},
		Webhook: WebhookConfig{Enabled: false},
	}

	return &Reconciler{
		Client:         fakeClient,
		Scheme:         scheme,
		Config:         &rest.Config{},
		EventRecorder:  record.NewFakeRecorder(100),
		config:         config,
		healthCheckers: nil,
	}
}

func TestUpdateConditions_NoPipelines(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	_ = operatorv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	obj := operatorv1alpha1.Telemetry{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Spec: operatorv1alpha1.TelemetrySpec{},
	}

	err := fakeClient.Create(ctx, &obj)
	require.NoError(t, err)
	rc := initReconciler(fakeClient)

	mockLogCompHealthChecker := &mocks.ComponentHealthChecker{}
	mockTraceCompHealthChecker := &mocks.ComponentHealthChecker{}
	mockMetricCompHealthChecker := &mocks.ComponentHealthChecker{}
	compHealthChecker := map[string]ComponentHealthChecker{
		"Log Components":     mockLogCompHealthChecker,
		"Trace Components":   mockTraceCompHealthChecker,
		"Metrics Components": mockMetricCompHealthChecker,
	}

	rc.healthCheckers = compHealthChecker

	mockLogCompHealthChecker.On("Check", mock.Anything, mock.Anything).Return(getLoggingCondition(reconciler.ConditionStatusTrue, reconciler.ReasonNoPipelineDeployed), nil)
	mockTraceCompHealthChecker.On("Check", mock.Anything, mock.Anything).Return(getTraceCondition(reconciler.ConditionStatusTrue, reconciler.ReasonNoPipelineDeployed), nil)
	mockMetricCompHealthChecker.On("Check", mock.Anything, mock.Anything).Return(getMetricCondition(reconciler.ConditionStatusTrue, reconciler.ReasonNoPipelineDeployed), nil)

	err = rc.updateStatus(ctx, &obj)
	require.NoError(t, err)
	conditions := obj.Status.Conditions
	require.Len(t, conditions, 3)
	endpoints := obj.Status.GatewayEndpoints
	expectedEndpoint := operatorv1alpha1.GatewayEndpoints{
		Traces:  &operatorv1alpha1.OTLPEndpoints{},
		Metrics: &operatorv1alpha1.OTLPEndpoints{},
	}
	require.Equal(t, endpoints, expectedEndpoint)
	var expectedState operatorv1alpha1.State = "Ready"
	require.Equal(t, obj.Status.Status.State, expectedState)
}

func TestUpdateConditions_LogPipelinePending(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	_ = operatorv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	obj := operatorv1alpha1.Telemetry{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Spec: operatorv1alpha1.TelemetrySpec{},
	}

	err := fakeClient.Create(ctx, &obj)
	require.NoError(t, err)
	rc := initReconciler(fakeClient)

	mockLogCompHealthChecker := &mocks.ComponentHealthChecker{}
	mockTraceCompHealthChecker := &mocks.ComponentHealthChecker{}
	mockMetricCompHealthChecker := &mocks.ComponentHealthChecker{}
	compHealthChecker := map[string]ComponentHealthChecker{
		"Log Components":     mockLogCompHealthChecker,
		"Trace Components":   mockTraceCompHealthChecker,
		"Metrics Components": mockMetricCompHealthChecker,
	}

	rc.healthCheckers = compHealthChecker
	mockLogCompHealthChecker.On("Check", mock.Anything, mock.Anything).Return(getLoggingCondition(reconciler.ConditionStatusFalse, reconciler.ReasonFluentBitDSNotReady), nil)
	mockTraceCompHealthChecker.On("Check", mock.Anything, mock.Anything).Return(getTraceCondition(reconciler.ConditionStatusTrue, reconciler.ReasonNoPipelineDeployed), nil)
	mockMetricCompHealthChecker.On("Check", mock.Anything, mock.Anything).Return(getMetricCondition(reconciler.ConditionStatusTrue, reconciler.ReasonNoPipelineDeployed), nil)

	err = rc.updateStatus(ctx, &obj)
	require.NoError(t, err)
	conditions := obj.Status.Conditions
	for _, c := range conditions {
		if c.Type == "Logging" {
			require.Equal(t, c.Reason, reconciler.ReasonFluentBitDSNotReady)
		}
	}
}

func TestUpdateConditions_TracePipelineRunning(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	_ = operatorv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	obj := operatorv1alpha1.Telemetry{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Spec: operatorv1alpha1.TelemetrySpec{},
	}

	traceObj := telemetryv1alpha1.TracePipeline{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{},
	}

	err := fakeClient.Create(ctx, &obj)
	require.NoError(t, err)

	err = fakeClient.Create(ctx, &traceObj)
	require.NoError(t, err)

	rc := initReconciler(fakeClient)

	mockLogCompHealthChecker := &mocks.ComponentHealthChecker{}
	mockTraceCompHealthChecker := &mocks.ComponentHealthChecker{}
	mockMetricCompHealthChecker := &mocks.ComponentHealthChecker{}
	compHealthChecker := map[string]ComponentHealthChecker{
		"Log Components":     mockLogCompHealthChecker,
		"Trace Components":   mockTraceCompHealthChecker,
		"Metrics Components": mockMetricCompHealthChecker,
	}

	rc.healthCheckers = compHealthChecker

	mockLogCompHealthChecker.On("Check", mock.Anything).Return(getLoggingCondition(reconciler.ConditionStatusTrue, reconciler.ReasonFluentBitDSReady), nil)
	mockTraceCompHealthChecker.On("Check", mock.Anything).Return(getTraceCondition(reconciler.ConditionStatusTrue, reconciler.ReasonTraceCollectorDeploymentReady), nil)
	mockMetricCompHealthChecker.On("Check", mock.Anything).Return(getMetricCondition(reconciler.ConditionStatusTrue, reconciler.ReasonNoPipelineDeployed), nil)

	err = rc.updateStatus(ctx, &obj)
	require.NoError(t, err)
	conditions := obj.Status.Conditions
	for _, c := range conditions {
		if c.Type == "Tracing" {
			require.Equal(t, c.Reason, reconciler.ReasonTraceCollectorDeploymentReady)
		}
	}
	endpoints := obj.Status.GatewayEndpoints
	require.Equal(t, endpoints.Traces.GRPC, "http://trace-otlp-svc.default:4317")
	require.Equal(t, endpoints.Traces.HTTP, "http://trace-otlp-svc.default:4318")
}

func TestUpdateConditions_MetricPipelineRunning(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	_ = operatorv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	obj := operatorv1alpha1.Telemetry{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Spec: operatorv1alpha1.TelemetrySpec{},
	}

	metricObj := telemetryv1alpha1.MetricPipeline{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: telemetryv1alpha1.MetricPipelineSpec{},
	}

	err := fakeClient.Create(ctx, &obj)
	require.NoError(t, err)

	err = fakeClient.Create(ctx, &metricObj)
	require.NoError(t, err)

	rc := initReconciler(fakeClient)

	mockLogCompHealthChecker := &mocks.ComponentHealthChecker{}
	mockTraceCompHealthChecker := &mocks.ComponentHealthChecker{}
	mockMetricCompHealthChecker := &mocks.ComponentHealthChecker{}
	compHealthChecker := map[string]ComponentHealthChecker{
		"Log Components":     mockLogCompHealthChecker,
		"Trace Components":   mockTraceCompHealthChecker,
		"Metrics Components": mockMetricCompHealthChecker,
	}

	rc.healthCheckers = compHealthChecker

	mockLogCompHealthChecker.On("Check", mock.Anything).Return(getLoggingCondition(reconciler.ConditionStatusTrue, reconciler.ReasonFluentBitDSReady), nil)
	mockTraceCompHealthChecker.On("Check", mock.Anything).Return(getTraceCondition(reconciler.ConditionStatusFalse, reconciler.ReasonNoPipelineDeployed), nil)
	mockMetricCompHealthChecker.On("Check", mock.Anything).Return(getMetricCondition(reconciler.ConditionStatusTrue, reconciler.ReasonMetricGatewayDeploymentReady), nil)

	err = rc.updateStatus(ctx, &obj)
	require.NoError(t, err)
	conditions := obj.Status.Conditions
	for _, c := range conditions {
		if c.Type == "Metrics" {
			require.Equal(t, c.Reason, reconciler.ReasonMetricGatewayDeploymentReady)
		}
	}
	endpoints := obj.Status.GatewayEndpoints
	require.Equal(t, endpoints.Metrics.GRPC, "http://metric-otlp-svc.default:4317")
	require.Equal(t, endpoints.Metrics.HTTP, "http://metric-otlp-svc.default:4318")
	require.Equal(t, endpoints.Traces.GRPC, "")
	require.Equal(t, endpoints.Traces.HTTP, "")
}

func TestUpdateConditions_TracePipelinePending(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	_ = operatorv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	obj := operatorv1alpha1.Telemetry{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Spec: operatorv1alpha1.TelemetrySpec{},
	}

	traceObj := telemetryv1alpha1.TracePipeline{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{},
	}

	err := fakeClient.Create(ctx, &obj)
	require.NoError(t, err)

	err = fakeClient.Create(ctx, &traceObj)
	require.NoError(t, err)

	rc := initReconciler(fakeClient)

	mockLogCompHealthChecker := &mocks.ComponentHealthChecker{}
	mockTraceCompHealthChecker := &mocks.ComponentHealthChecker{}
	mockMetricCompHealthChecker := &mocks.ComponentHealthChecker{}
	compHealthChecker := map[string]ComponentHealthChecker{
		"Log Components":     mockLogCompHealthChecker,
		"Trace Components":   mockTraceCompHealthChecker,
		"Metrics Components": mockMetricCompHealthChecker,
	}

	rc.healthCheckers = compHealthChecker

	mockLogCompHealthChecker.On("Check", mock.Anything).Return(getLoggingCondition(reconciler.ConditionStatusTrue, reconciler.ReasonFluentBitDSReady), nil)
	mockTraceCompHealthChecker.On("Check", mock.Anything).Return(getTraceCondition(reconciler.ConditionStatusFalse, reconciler.ReasonTraceCollectorDeploymentNotReady), nil)
	mockMetricCompHealthChecker.On("Check", mock.Anything).Return(getMetricCondition(reconciler.ConditionStatusTrue, reconciler.ReasonNoPipelineDeployed), nil)

	err = rc.updateStatus(ctx, &obj)
	require.NoError(t, err)
	state := obj.Status.Status.State
	var expectedState operatorv1alpha1.State = "Warning"
	require.Equal(t, state, expectedState)

}

func TestUpdateConditions_CheckWarningState(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	_ = operatorv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	obj := operatorv1alpha1.Telemetry{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Spec: operatorv1alpha1.TelemetrySpec{},
	}

	logObj := telemetryv1alpha1.LogPipeline{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{},
	}

	err := fakeClient.Create(ctx, &obj)
	require.NoError(t, err)

	err = fakeClient.Create(ctx, &logObj)
	require.NoError(t, err)

	rc := initReconciler(fakeClient)

	mockLogCompHealthChecker := &mocks.ComponentHealthChecker{}
	mockTraceCompHealthChecker := &mocks.ComponentHealthChecker{}
	mockMetricCompHealthChecker := &mocks.ComponentHealthChecker{}
	compHealthChecker := map[string]ComponentHealthChecker{
		"Log Components":     mockLogCompHealthChecker,
		"Trace Components":   mockTraceCompHealthChecker,
		"Metrics Components": mockMetricCompHealthChecker,
	}

	rc.healthCheckers = compHealthChecker

	mockLogCompHealthChecker.On("Check", mock.Anything).Return(getLoggingCondition(reconciler.ConditionStatusFalse, reconciler.ReasonReferencedSecretMissing), nil)
	mockTraceCompHealthChecker.On("Check", mock.Anything).Return(getTraceCondition(reconciler.ConditionStatusFalse, reconciler.ReasonTraceCollectorDeploymentNotReady), nil)
	mockMetricCompHealthChecker.On("Check", mock.Anything).Return(getMetricCondition(reconciler.ConditionStatusFalse, reconciler.ReasonReferencedSecretMissing), nil)

	err = rc.updateStatus(ctx, &obj)
	require.NoError(t, err)
	conditions := obj.Status.Conditions
	for _, c := range conditions {
		if c.Type == "Tracing" {
			require.Equal(t, c.Reason, reconciler.ReasonTraceCollectorDeploymentNotReady)
		}
	}
	endpoints := obj.Status.GatewayEndpoints
	require.Equal(t, endpoints.Traces.GRPC, "")
	require.Equal(t, endpoints.Traces.HTTP, "")
}

func getLoggingCondition(status metav1.ConditionStatus, reason string) *metav1.Condition {
	return &metav1.Condition{
		Type:               "Logging",
		Status:             status,
		ObservedGeneration: 1,
		Reason:             reason,
		Message:            reconciler.Conditions[reason],
	}
}
func getMetricCondition(status metav1.ConditionStatus, reason string) *metav1.Condition {
	return &metav1.Condition{
		Type:               "Metrics",
		Status:             status,
		ObservedGeneration: 1,
		Reason:             reason,
		Message:            reconciler.Conditions[reason],
	}
}
func getTraceCondition(status metav1.ConditionStatus, reason string) *metav1.Condition {
	return &metav1.Condition{
		Type:               "Tracing",
		Status:             status,
		ObservedGeneration: 1,
		Reason:             reason,
		Message:            reconciler.Conditions[reason],
	}
}
