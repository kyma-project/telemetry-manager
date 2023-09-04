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
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestMetricComponentsCheck(t *testing.T) {
	tests := []struct {
		name              string
		pipelines         []telemetryv1alpha1.MetricPipeline
		expectedCondition *metav1.Condition
	}{
		{
			name: "should be healthy if no pipelines deployed",
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "True",
				Reason:  "NoPipelineDeployed",
				Message: "No pipelines have been deployed",
			},
		},
		{
			name: "should be healthy if all pipelines running",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithStatusConditions(
					testutils.MetricPendingCondition(reconciler.ReasonMetricGatewayDeploymentNotReady), testutils.MetricRunningCondition()).Build(),
				testutils.NewMetricPipelineBuilder().WithStatusConditions(
					testutils.MetricPendingCondition(reconciler.ReasonMetricGatewayDeploymentNotReady), testutils.MetricRunningCondition()).Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "True",
				Reason:  "MetricGatewayDeploymentReady",
				Message: "Metric gateway Deployment is ready",
			},
		},
		{
			name: "should fail if one pipeline refs missing secret",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithStatusConditions(
					testutils.MetricPendingCondition(reconciler.ReasonMetricGatewayDeploymentNotReady), testutils.MetricRunningCondition()).Build(),
				testutils.NewMetricPipelineBuilder().WithStatusConditions(
					testutils.MetricPendingCondition(reconciler.ReasonReferencedSecretMissing)).Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "False",
				Reason:  "ReferencedSecretMissing",
				Message: "One or more referenced Secrets are missing",
			},
		},
		{
			name: "should fail if one pipeline waiting for gateway",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithStatusConditions(
					testutils.MetricPendingCondition(reconciler.ReasonMetricGatewayDeploymentNotReady), testutils.MetricRunningCondition()).Build(),
				testutils.NewMetricPipelineBuilder().WithStatusConditions(
					testutils.MetricPendingCondition(reconciler.ReasonMetricGatewayDeploymentNotReady)).Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "False",
				Reason:  "MetricGatewayDeploymentNotReady",
				Message: "Metric gateway Deployment is not ready",
			},
		},
		{
			name: "should ignore pipelines waiting for lock",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithStatusConditions(
					testutils.MetricPendingCondition(reconciler.ReasonMetricGatewayDeploymentNotReady), testutils.MetricRunningCondition()).Build(),
				testutils.NewMetricPipelineBuilder().WithStatusConditions(
					testutils.MetricPendingCondition(reconciler.ReasonWaitingForLock)).Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "True",
				Reason:  "MetricGatewayDeploymentReady",
				Message: "Metric gateway Deployment is ready",
			},
		},
		{
			name: "should prioritize missing secret over unready gateway reason",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithStatusConditions(
					testutils.MetricPendingCondition(reconciler.ReasonMetricGatewayDeploymentNotReady)).Build(),
				testutils.NewMetricPipelineBuilder().WithStatusConditions(
					testutils.MetricPendingCondition(reconciler.ReasonReferencedSecretMissing)).Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "False",
				Reason:  "MetricGatewayDeploymentNotReady",
				Message: "Metric gateway Deployment is not ready",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			b := fake.NewClientBuilder().WithScheme(scheme)
			for i := range test.pipelines {
				b.WithObjects(&test.pipelines[i])
			}
			fakeClient := b.Build()

			m := &metricComponentsChecker{
				client: fakeClient,
			}

			condition, err := m.Check(context.Background())
			require.NoError(t, err)
			require.Equal(t, test.expectedCondition, condition)
		})
	}
}
