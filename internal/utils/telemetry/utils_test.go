package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
)

func TestDefaultTelemetryInstanceFound(t *testing.T) {
	ctx := t.Context()
	scheme := runtime.NewScheme()
	_ = operatorv1alpha1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&operatorv1alpha1.Telemetry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultTelemetryInstanceName,
			Namespace: "default",
		},
	}).Build()

	telemetry, err := GetDefaultTelemetryInstance(ctx, client, "default")
	require.NoError(t, err)
	assert.Equal(t, DefaultTelemetryInstanceName, telemetry.Name)
}

func TestDefaultTelemetryInstanceNotFound(t *testing.T) {
	ctx := t.Context()
	client := fake.NewClientBuilder().Build()

	_, err := GetDefaultTelemetryInstance(ctx, client, "default")
	assert.Error(t, err)
}

func TestGetEnrichmentsFromTelemetryWithValidConfig(t *testing.T) {
	ctx := t.Context()
	scheme := runtime.NewScheme()
	_ = operatorv1alpha1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&operatorv1alpha1.Telemetry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultTelemetryInstanceName,
			Namespace: "default",
		},
		Spec: operatorv1alpha1.TelemetrySpec{
			Enrichments: &operatorv1alpha1.EnrichmentSpec{
				ExtractPodLabels: []operatorv1alpha1.PodLabel{
					{Key: "app", KeyPrefix: "prefix"},
				},
			},
		},
	}).Build()

	enrichments := GetEnrichmentsFromTelemetry(ctx, client, "default")
	require.Len(t, enrichments.PodLabels, 1)
	assert.Equal(t, "app", enrichments.PodLabels[0].Key)
	assert.Equal(t, "prefix", enrichments.PodLabels[0].KeyPrefix)
}

func TestGetEnrichmentsFromTelemetryWithNoConfig(t *testing.T) {
	ctx := t.Context()
	scheme := runtime.NewScheme()
	_ = operatorv1alpha1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&operatorv1alpha1.Telemetry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultTelemetryInstanceName,
			Namespace: "default",
		},
	}).Build()

	enrichments := GetEnrichmentsFromTelemetry(ctx, client, "default")
	assert.Empty(t, enrichments.PodLabels)
}

func TestGetEnrichmentsFromTelemetryErrorOnMissingInstance(t *testing.T) {
	ctx := t.Context()
	client := fake.NewClientBuilder().Build()

	enrichments := GetEnrichmentsFromTelemetry(ctx, client, "default")
	assert.Empty(t, enrichments.PodLabels)
}
