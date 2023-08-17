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

func TestLogPipelineMissingSecret(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	lc := logComponentsChecker{client: fakeClient}
	logObj := makeLogPipeline("foo", telemetryv1alpha1.LogPipelinePending, reconciler.ReasonReferencedSecretMissing)

	err := fakeClient.Create(ctx, &logObj)
	require.NoError(t, err)

	cond, err := lc.Check(ctx)
	require.NoError(t, err)
	expectedCond := &metav1.Condition{
		Type:    "LogComponentsHealthy",
		Status:  "False",
		Reason:  "ReferencedSecretMissing",
		Message: "One or more referenced secrets are missing",
	}
	require.Equal(t, cond, expectedCond)

}

func TestMultipleLogPipelineOnePending(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	lc := logComponentsChecker{client: fakeClient}
	logObj0 := makeLogPipeline("foo", telemetryv1alpha1.LogPipelinePending, reconciler.ReasonFluentBitDSNotReady)
	logObj1 := makeLogPipeline("bar", telemetryv1alpha1.LogPipelineRunning, reconciler.ReasonFluentBitDSReady)

	err := fakeClient.Create(ctx, &logObj0)
	require.NoError(t, err)
	err = fakeClient.Create(ctx, &logObj1)
	require.NoError(t, err)

	cond, err := lc.Check(ctx)
	require.NoError(t, err)
	expectedCond := &metav1.Condition{
		Type:    "LogComponentsHealthy",
		Status:  "False",
		Reason:  "FluentBitDaemonSetNotReady",
		Message: "Fluent bit Daemonset is not ready",
	}
	require.Equal(t, cond, expectedCond)

}

func TestAllLogPipelinesHealthy(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	lc := logComponentsChecker{client: fakeClient}

	logObj0 := makeLogPipeline("foo", telemetryv1alpha1.LogPipelineRunning, reconciler.ReasonFluentBitDSReady)
	logObj1 := makeLogPipeline("bar", telemetryv1alpha1.LogPipelineRunning, reconciler.ReasonFluentBitDSReady)

	err := fakeClient.Create(ctx, &logObj0)
	require.NoError(t, err)
	err = fakeClient.Create(ctx, &logObj1)
	require.NoError(t, err)

	cond, err := lc.Check(ctx)
	require.NoError(t, err)
	expectedCond := &metav1.Condition{
		Type:    "LogComponentsHealthy",
		Status:  "True",
		Reason:  "FluentBitDaemonSetReady",
		Message: "Fluent bit Daemonset is ready",
	}
	require.Equal(t, cond, expectedCond)

}

func makeLogPipeline(name string, state telemetryv1alpha1.LogPipelineConditionType, reason string) telemetryv1alpha1.LogPipeline {
	return telemetryv1alpha1.LogPipeline{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{},
		Status: telemetryv1alpha1.LogPipelineStatus{
			Conditions: []telemetryv1alpha1.LogPipelineCondition{{
				Type:   state,
				Reason: reason},
			},
			UnsupportedMode: false,
		},
	}
}
