package storagemigration

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestNeedsMigration(t *testing.T) {
	tests := []struct {
		name           string
		storedVersions []string
		expectedResult bool
	}{
		{
			name:           "needs migration when v1alpha1 present",
			storedVersions: []string{"v1alpha1", "v1beta1"},
			expectedResult: true,
		},
		{
			name:           "no migration needed when only v1beta1",
			storedVersions: []string{"v1beta1"},
			expectedResult: false,
		},
		{
			name:           "needs migration when v1alpha1 is only version",
			storedVersions: []string{"v1alpha1"},
			expectedResult: true,
		},
		{
			name:           "no migration needed when empty",
			storedVersions: []string{},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme(t)
			crd := newTestCRD("logpipelines.telemetry.kyma-project.io", tt.storedVersions)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(crd).
				Build()

			migrator := New(fakeClient, logr.Discard())

			result, err := migrator.needsMigration(context.Background(), crd.Name)
			require.NoError(t, err)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestClearStoredVersion(t *testing.T) {
	tests := []struct {
		name                   string
		initialStoredVersions  []string
		expectedStoredVersions []string
	}{
		{
			name:                   "removes v1alpha1 when both versions present",
			initialStoredVersions:  []string{"v1alpha1", "v1beta1"},
			expectedStoredVersions: []string{"v1beta1"},
		},
		{
			name:                   "no change when v1alpha1 not present",
			initialStoredVersions:  []string{"v1beta1"},
			expectedStoredVersions: []string{"v1beta1"},
		},
		{
			name:                   "removes v1alpha1 when only version",
			initialStoredVersions:  []string{"v1alpha1"},
			expectedStoredVersions: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme(t)
			crd := newTestCRD("logpipelines.telemetry.kyma-project.io", tt.initialStoredVersions)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(crd).
				WithStatusSubresource(crd).
				Build()

			migrator := New(fakeClient, logr.Discard())

			err := migrator.clearStoredVersion(context.Background(), crd.Name)
			require.NoError(t, err)

			var updatedCRD apiextensionsv1.CustomResourceDefinition

			err = fakeClient.Get(context.Background(), types.NamespacedName{Name: crd.Name}, &updatedCRD)
			require.NoError(t, err)
			require.Equal(t, tt.expectedStoredVersions, updatedCRD.Status.StoredVersions)
		})
	}
}

func TestMigrateLogPipelines(t *testing.T) {
	scheme := newTestScheme(t)

	logPipeline := &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-log-pipeline",
		},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Output: telemetryv1beta1.LogPipelineOutput{
				FluentBitCustom: "test-output",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(logPipeline).
		Build()

	migrator := New(fakeClient, logr.Discard())

	err := migrator.migrateLogPipelines(context.Background())
	require.NoError(t, err)

	var updatedPipeline telemetryv1beta1.LogPipeline

	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: logPipeline.Name}, &updatedPipeline)
	require.NoError(t, err)
	require.Equal(t, logPipeline.Name, updatedPipeline.Name)
}

func TestMigrateMetricPipelines(t *testing.T) {
	scheme := newTestScheme(t)

	metricPipeline := &telemetryv1beta1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-metric-pipeline",
		},
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Output: telemetryv1beta1.MetricPipelineOutput{
				OTLP: &telemetryv1beta1.OTLPOutput{
					Endpoint: telemetryv1beta1.ValueType{Value: "http://example.com"},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(metricPipeline).
		Build()

	migrator := New(fakeClient, logr.Discard())

	err := migrator.migrateMetricPipelines(context.Background())
	require.NoError(t, err)

	var updatedPipeline telemetryv1beta1.MetricPipeline

	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: metricPipeline.Name}, &updatedPipeline)
	require.NoError(t, err)
	require.Equal(t, metricPipeline.Name, updatedPipeline.Name)
}

func TestMigrateTracePipelines(t *testing.T) {
	scheme := newTestScheme(t)

	tracePipeline := &telemetryv1beta1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-trace-pipeline",
		},
		Spec: telemetryv1beta1.TracePipelineSpec{
			Output: telemetryv1beta1.TracePipelineOutput{
				OTLP: &telemetryv1beta1.OTLPOutput{
					Endpoint: telemetryv1beta1.ValueType{Value: "http://example.com"},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(tracePipeline).
		Build()

	migrator := New(fakeClient, logr.Discard())

	err := migrator.migrateTracePipelines(context.Background())
	require.NoError(t, err)

	var updatedPipeline telemetryv1beta1.TracePipeline

	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: tracePipeline.Name}, &updatedPipeline)
	require.NoError(t, err)
	require.Equal(t, tracePipeline.Name, updatedPipeline.Name)
}

func TestMigrateTelemetries(t *testing.T) {
	scheme := newTestScheme(t)

	telemetry := &operatorv1beta1.Telemetry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "kyma-system",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(telemetry).
		Build()

	migrator := New(fakeClient, logr.Discard())

	err := migrator.migrateTelemetries(context.Background())
	require.NoError(t, err)

	var updatedTelemetry operatorv1beta1.Telemetry

	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: telemetry.Name, Namespace: telemetry.Namespace}, &updatedTelemetry)
	require.NoError(t, err)
	require.Equal(t, telemetry.Name, updatedTelemetry.Name)
}

func TestMigrateIfNeeded_NoMigrationNeeded(t *testing.T) {
	scheme := newTestScheme(t)

	// All CRDs already have only v1beta1
	logCRD := newTestCRD("logpipelines.telemetry.kyma-project.io", []string{"v1beta1"})
	metricCRD := newTestCRD("metricpipelines.telemetry.kyma-project.io", []string{"v1beta1"})
	traceCRD := newTestCRD("tracepipelines.telemetry.kyma-project.io", []string{"v1beta1"})
	telemetryCRDObj := newTestCRD(telemetryCRD, []string{"v1beta1"})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(logCRD, metricCRD, traceCRD, telemetryCRDObj).
		WithStatusSubresource(logCRD, metricCRD, traceCRD, telemetryCRDObj).
		Build()

	migrator := New(fakeClient, logr.Discard())

	err := migrator.MigrateIfNeeded(context.Background())
	require.NoError(t, err)

	// Verify storedVersions unchanged for pipeline CRDs
	for _, crdName := range pipelineCRDs {
		var crd apiextensionsv1.CustomResourceDefinition

		err := fakeClient.Get(context.Background(), types.NamespacedName{Name: crdName}, &crd)
		require.NoError(t, err)
		require.Equal(t, []string{"v1beta1"}, crd.Status.StoredVersions)
	}

	// Verify storedVersions unchanged for Telemetry CRD
	var telemetryCRDResult apiextensionsv1.CustomResourceDefinition

	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: telemetryCRD}, &telemetryCRDResult)
	require.NoError(t, err)
	require.Equal(t, []string{"v1beta1"}, telemetryCRDResult.Status.StoredVersions)
}

func TestMigrateIfNeeded_MigrationPerformed(t *testing.T) {
	scheme := newTestScheme(t)

	// CRDs have both v1alpha1 and v1beta1
	logCRD := newTestCRD("logpipelines.telemetry.kyma-project.io", []string{"v1alpha1", "v1beta1"})
	metricCRD := newTestCRD("metricpipelines.telemetry.kyma-project.io", []string{"v1alpha1", "v1beta1"})
	traceCRD := newTestCRD("tracepipelines.telemetry.kyma-project.io", []string{"v1alpha1", "v1beta1"})
	telemetryCRDObj := newTestCRD(telemetryCRD, []string{"v1alpha1", "v1beta1"})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(logCRD, metricCRD, traceCRD, telemetryCRDObj).
		WithStatusSubresource(logCRD, metricCRD, traceCRD, telemetryCRDObj).
		Build()

	migrator := New(fakeClient, logr.Discard())

	err := migrator.MigrateIfNeeded(context.Background())
	require.NoError(t, err)

	// Verify v1alpha1 was removed from storedVersions for pipeline CRDs
	for _, crdName := range pipelineCRDs {
		var crd apiextensionsv1.CustomResourceDefinition

		err := fakeClient.Get(context.Background(), types.NamespacedName{Name: crdName}, &crd)
		require.NoError(t, err)
		require.Equal(t, []string{"v1beta1"}, crd.Status.StoredVersions)
	}

	// Verify v1alpha1 was removed from storedVersions for Telemetry CRD
	var telemetryCRDResult apiextensionsv1.CustomResourceDefinition

	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: telemetryCRD}, &telemetryCRDResult)
	require.NoError(t, err)
	require.Equal(t, []string{"v1beta1"}, telemetryCRDResult.Status.StoredVersions)
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))
	require.NoError(t, telemetryv1beta1.AddToScheme(scheme))
	require.NoError(t, operatorv1beta1.AddToScheme(scheme))

	return scheme
}

func newTestCRD(name string, storedVersions []string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "telemetry.kyma-project.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "resources",
				Singular: "resource",
				Kind:     "Resource",
			},
			Scope: apiextensionsv1.ClusterScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true,
				},
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: false,
				},
			},
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			StoredVersions: storedVersions,
		},
	}
}
