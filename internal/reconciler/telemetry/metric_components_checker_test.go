package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
)

func TestMetricPipelineMissingSecret(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	mc := metricComponentsChecker{client: fakeClient}
	metricObj := getMetricPipeline("foo", telemetryv1alpha1.MetricPipelinePending, reconciler.ReasonReferencedSecretMissing)

	err := fakeClient.Create(ctx, &metricObj)
	require.NoError(t, err)

	cond, err := mc.Check(ctx)
	require.NoError(t, err)
	expectedCond := &metav1.Condition{
		Type:    reconciler.MetricConditionType,
		Status:  reconciler.ConditionStatusFalse,
		Reason:  reconciler.ReasonReferencedSecretMissing,
		Message: "One or more referenced secrets are missing",
	}
	require.Equal(t, cond, expectedCond)

}

func TestMultipleMetricPipelineOnePending(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	mc := metricComponentsChecker{client: fakeClient}
	metricObj0 := getMetricPipeline("foo", telemetryv1alpha1.MetricPipelinePending, reconciler.ReasonMetricGatewayDeploymentNotReady)
	metricObj1 := getMetricPipeline("bar", telemetryv1alpha1.MetricPipelineRunning, reconciler.ReasonMetricGatewayDeploymentReady)

	err := fakeClient.Create(ctx, &metricObj0)
	require.NoError(t, err)
	err = fakeClient.Create(ctx, &metricObj1)
	require.NoError(t, err)

	cond, err := mc.Check(ctx)
	require.NoError(t, err)
	expectedCond := &metav1.Condition{
		Type:    reconciler.MetricConditionType,
		Status:  reconciler.ConditionStatusFalse,
		Reason:  reconciler.ReasonMetricGatewayDeploymentNotReady,
		Message: "Metric gateway deployment is not ready",
	}
	require.Equal(t, cond, expectedCond)

}

func TestAllMetricPipelinesHealthy(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	mc := metricComponentsChecker{client: fakeClient}

	metricObj0 := getMetricPipeline("foo", telemetryv1alpha1.MetricPipelineRunning, reconciler.ReasonMetricGatewayDeploymentReady)
	metricObj1 := getMetricPipeline("bar", telemetryv1alpha1.MetricPipelineRunning, reconciler.ReasonMetricGatewayDeploymentReady)

	err := fakeClient.Create(ctx, &metricObj0)
	require.NoError(t, err)
	err = fakeClient.Create(ctx, &metricObj1)
	require.NoError(t, err)

	cond, err := mc.Check(ctx)
	require.NoError(t, err)
	expectedCond := &metav1.Condition{
		Type:    reconciler.MetricConditionType,
		Status:  reconciler.ConditionStatusTrue,
		Reason:  reconciler.ReasonMetricGatewayDeploymentReady,
		Message: "Metric gateway deployment is ready",
	}
	require.Equal(t, cond, expectedCond)

}

func TestMultipleMetricPipelinesOneLock(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	mc := metricComponentsChecker{client: fakeClient}

	metricObj0 := getMetricPipeline("foo", telemetryv1alpha1.MetricPipelineRunning, reconciler.ReasonMetricGatewayDeploymentReady)
	metricObj1 := getMetricPipeline("bar", telemetryv1alpha1.MetricPipelinePending, reconciler.ReasonWaitingForLock)

	err := fakeClient.Create(ctx, &metricObj0)
	require.NoError(t, err)
	err = fakeClient.Create(ctx, &metricObj1)
	require.NoError(t, err)

	cond, err := mc.Check(ctx)
	require.NoError(t, err)
	expectedCond := &metav1.Condition{
		Type:    reconciler.MetricConditionType,
		Status:  reconciler.ConditionStatusTrue,
		Reason:  reconciler.ReasonMetricGatewayDeploymentReady,
		Message: "Metric gateway deployment is ready",
	}
	require.Equal(t, cond, expectedCond)

}

func getMetricPipeline(name string, state telemetryv1alpha1.MetricPipelineConditionType, reason string) telemetryv1alpha1.MetricPipeline {
	return telemetryv1alpha1.MetricPipeline{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: telemetryv1alpha1.MetricPipelineSpec{},
		Status: telemetryv1alpha1.MetricPipelineStatus{
			Conditions: []telemetryv1alpha1.MetricPipelineCondition{{
				Type:   state,
				Reason: reason},
			},
		},
	}
}
