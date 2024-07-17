package workloadstatus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeploymentProber_IsReady(t *testing.T) {
	tests := []struct {
		summary          string
		desiredScheduled *int32
		numberReady      int32
		expected         bool
	}{
		{summary: "all scheduled all ready", desiredScheduled: ptr.To(int32(1)), numberReady: 1, expected: true},
		{summary: "all scheduled one ready", desiredScheduled: ptr.To(int32(2)), numberReady: 1, expected: false},
		{summary: "all scheduled zero ready", desiredScheduled: ptr.To(int32(1)), numberReady: 0, expected: false},
	}

	for _, test := range tests {
		t.Run(test.summary, func(t *testing.T) {

			t.Parallel()

			matchLabels := make(map[string]string)
			matchLabels["test.deployment.name"] = "test-deployment"

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "telemetry-system"},
				Spec: appsv1.DeploymentSpec{
					Replicas: test.desiredScheduled,
					Selector: &metav1.LabelSelector{MatchLabels: matchLabels},
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: test.numberReady,
				},
			}

			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "foo",
					Namespace:       "telemetry-system",
					Labels:          deployment.Spec.Selector.MatchLabels,
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(deployment, deployment.GroupVersionKind())},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: deployment.Spec.Selector,
					Replicas: test.desiredScheduled,
					Template: deployment.Spec.Template,
				},
				Status: appsv1.ReplicaSetStatus{
					ReadyReplicas: test.numberReady,
					Replicas:      test.numberReady,
				},
			}

			itemList := make([]appsv1.ReplicaSet, 1)

			itemList = append(itemList, *rs)
			rsList := &appsv1.ReplicaSetList{
				Items: itemList,
			}

			fakeClient := fake.NewClientBuilder().WithObjects(deployment).WithLists(rsList).Build()

			sut := DeploymentProber{fakeClient}
			ready, err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})

			require.NoError(t, err)
			require.Equal(t, test.expected, ready)

		})
	}
}
