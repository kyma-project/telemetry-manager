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

func TestTracePipelineMissingSecret(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	tc := traceComponentsHealthChecker{client: fakeClient}
	metricObj := getTracePipeline("foo", telemetryv1alpha1.TracePipelinePending, reconciler.ReasonReferencedSecretMissing)

	err := fakeClient.Create(ctx, &metricObj)
	require.NoError(t, err)

	cond, err := tc.Check(ctx)
	require.NoError(t, err)
	expectedCond := &metav1.Condition{
		Type:    reconciler.TraceConditionType,
		Status:  reconciler.ConditionStatusFalse,
		Reason:  reconciler.ReasonReferencedSecretMissing,
		Message: "One or more referenced secrets are missing",
	}
	require.Equal(t, cond, expectedCond)

}

func TestMultipleTracePipelineOnePending(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	tc := traceComponentsHealthChecker{client: fakeClient}
	traceObj0 := getTracePipeline("foo", telemetryv1alpha1.TracePipelinePending, reconciler.ReasonMetricGatewayDeploymentNotReady)
	traceObj1 := getTracePipeline("bar", telemetryv1alpha1.TracePipelineRunning, reconciler.ReasonMetricGatewayDeploymentReady)

	err := fakeClient.Create(ctx, &traceObj0)
	require.NoError(t, err)
	err = fakeClient.Create(ctx, &traceObj1)
	require.NoError(t, err)

	cond, err := tc.Check(ctx)
	require.NoError(t, err)
	expectedCond := &metav1.Condition{
		Type:    reconciler.TraceConditionType,
		Status:  reconciler.ConditionStatusFalse,
		Reason:  reconciler.ReasonTraceCollectorDeploymentNotReady,
		Message: "Trace collector is deployment not ready",
	}
	require.Equal(t, cond, expectedCond)

}

func TestAllTracePipelinesHealthy(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	tc := traceComponentsHealthChecker{client: fakeClient}

	traceObj0 := getTracePipeline("foo", telemetryv1alpha1.TracePipelineRunning, reconciler.ReasonMetricGatewayDeploymentReady)
	traceObj1 := getTracePipeline("bar", telemetryv1alpha1.TracePipelineRunning, reconciler.ReasonMetricGatewayDeploymentReady)

	err := fakeClient.Create(ctx, &traceObj0)
	require.NoError(t, err)
	err = fakeClient.Create(ctx, &traceObj1)
	require.NoError(t, err)

	cond, err := tc.Check(ctx)
	require.NoError(t, err)
	expectedCond := &metav1.Condition{
		Type:    reconciler.TraceConditionType,
		Status:  reconciler.ConditionStatusTrue,
		Reason:  reconciler.ReasonTraceCollectorDeploymentReady,
		Message: "Trace collector deployment is ready",
	}
	require.Equal(t, cond, expectedCond)

}
func TestMultipleTracePipelinesOneLock(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	tc := traceComponentsHealthChecker{client: fakeClient}

	traceObj0 := getTracePipeline("foo", telemetryv1alpha1.TracePipelineRunning, reconciler.ReasonMetricGatewayDeploymentReady)
	traceObj1 := getTracePipeline("bar", telemetryv1alpha1.TracePipelinePending, reconciler.ReasonWaitingForLock)

	err := fakeClient.Create(ctx, &traceObj0)
	require.NoError(t, err)
	err = fakeClient.Create(ctx, &traceObj1)
	require.NoError(t, err)

	cond, err := tc.Check(ctx)
	require.NoError(t, err)
	expectedCond := &metav1.Condition{
		Type:    reconciler.TraceConditionType,
		Status:  reconciler.ConditionStatusTrue,
		Reason:  reconciler.ReasonTraceCollectorDeploymentReady,
		Message: "Trace collector deployment is ready",
	}
	require.Equal(t, cond, expectedCond)

}

func getTracePipeline(name string, state telemetryv1alpha1.TracePipelineConditionType, reason string) telemetryv1alpha1.TracePipeline {
	return telemetryv1alpha1.TracePipeline{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{},
		Status: telemetryv1alpha1.TracePipelineStatus{
			Conditions: []telemetryv1alpha1.TracePipelineCondition{{
				Type:   state,
				Reason: reason},
			},
		},
	}
}
