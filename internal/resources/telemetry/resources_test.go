package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
)

func TestDefaultTelemetryInstanceFound(t *testing.T) {
	ctx := context.Background()
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
	ctx := context.TODO()
	client := fake.NewClientBuilder().Build()

	_, err := GetDefaultTelemetryInstance(ctx, client, "default")
	assert.Error(t, err)
}
