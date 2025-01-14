package k8sutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSetAnnotation(t *testing.T) {
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "telemetry-system"},
	}

	fakeClient := fake.NewClientBuilder().WithObjects(daemonSet).Build()

	sut := DaemonSetAnnotator{fakeClient}

	err := sut.SetAnnotation(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"}, "foo", "bar")
	require.NoError(t, err)

	var updatedDaemonSet appsv1.DaemonSet
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"}, &updatedDaemonSet)
	require.Len(t, updatedDaemonSet.Spec.Template.Annotations, 1)
	require.Contains(t, updatedDaemonSet.Spec.Template.Annotations, "foo")
	require.Equal(t, updatedDaemonSet.Spec.Template.Annotations["foo"], "bar")
}
