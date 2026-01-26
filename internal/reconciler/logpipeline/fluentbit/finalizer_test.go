package fluentbit

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// TODO: remove tests after rollout telemetry 1.57.0
func TestCleanupFinalizers(t *testing.T) {
	t.Run("without files", func(t *testing.T) {
		ts := metav1.Now()
		pipeline := &telemetryv1beta1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "pipeline",
				Finalizers:        []string{sectionsFinalizer},
				DeletionTimestamp: &ts,
			},
		}

		scheme := runtime.NewScheme()
		_ = telemetryv1beta1.AddToScheme(scheme)
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).Build()

		err := cleanupFinalizers(t.Context(), client, pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1beta1.LogPipeline

		_ = client.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		require.False(t, controllerutil.ContainsFinalizer(&updatedPipeline, sectionsFinalizer))
	})

	t.Run("with files", func(t *testing.T) {
		ts := metav1.Now()
		pipeline := &telemetryv1beta1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "pipeline",
				Finalizers:        []string{sectionsFinalizer, filesFinalizer},
				DeletionTimestamp: &ts,
			},
		}

		scheme := runtime.NewScheme()
		_ = telemetryv1beta1.AddToScheme(scheme)
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).Build()

		err := cleanupFinalizers(t.Context(), client, pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1beta1.LogPipeline

		_ = client.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		require.False(t, controllerutil.ContainsFinalizer(&updatedPipeline, sectionsFinalizer))
		require.False(t, controllerutil.ContainsFinalizer(&updatedPipeline, filesFinalizer))
	})
}
