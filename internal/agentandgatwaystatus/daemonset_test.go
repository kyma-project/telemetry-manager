package agentandgatwaystatus

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestDaemonSetProber(t *testing.T) {
	tests := []struct {
		summary            string
		updatedScheduled   int32
		desiredScheduled   int32
		numberReady        int32
		observedGeneration int64
		desiredGeneration  int64

		oomError       bool
		crashBackError bool
		evictedError   bool
		pendingError   bool

		pods []*corev1.Pod

		expected      bool
		expectedError error
	}{
		{summary: "all scheduled all ready", desiredScheduled: 3, numberReady: 3, updatedScheduled: 3, expected: true},

		{
			summary:          "all scheduled one ready others have no problem",
			desiredScheduled: 3,
			numberReady:      1,
			updatedScheduled: 3,
			expected:         true,
			pods: []*corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).Build(),
			},
		}, // change to true
		{summary: "all scheduled one ready one crashbackloop with expired threshold", desiredScheduled: 3, numberReady: 1, updatedScheduled: 3, expected: true},
		{summary: "all scheduled one ready evicted", desiredScheduled: 3, numberReady: 1, updatedScheduled: 3, expected: true},
		{summary: "all scheduled one ready one OOM without expired threshold", desiredScheduled: 3, numberReady: 1, updatedScheduled: 3, expected: true},
		{summary: "all scheduled one ready one pending with expired threshold", desiredScheduled: 3, numberReady: 1, updatedScheduled: 3, expected: true},
		{summary: "all scheduled one ready one OOM and one Pending", desiredScheduled: 3, numberReady: 1, updatedScheduled: 3, expected: true},

		{summary: "all scheduled zero ready with no problem", desiredScheduled: 3, numberReady: 0, updatedScheduled: 3, expected: false},
		{summary: "all scheduled zero ready OOM,Pending,Crashback loop with expired threshold", desiredScheduled: 3, numberReady: 0, updatedScheduled: 3, expected: false},

		//{summary: "scheduled mismatch", desiredScheduled: 1, numberReady: 3, updatedScheduled: 3, expected: false}, // check for this condition
		//{summary: "desired scheduled mismatch", desiredScheduled: 3, numberReady: 3, updatedScheduled: 1, expected: false},
		//{summary: "generation mismatch", observedGeneration: 1, desiredGeneration: 2, expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.summary, func(t *testing.T) {
			t.Parallel()

			daemonSet := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "telemetry-system", Generation: tc.desiredGeneration},
				Spec: appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "foo"},
				}},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: tc.desiredScheduled,
					NumberReady:            tc.numberReady,
					UpdatedNumberScheduled: tc.updatedScheduled,
					ObservedGeneration:     tc.observedGeneration,
				},
			}

			pods := createPodList(tc.numberReady, daemonSet.Spec.Selector.MatchLabels)
			fakeClient := fake.NewClientBuilder()
			f := fakeClient.WithObjects(daemonSet)
			for _, pod := range pods {
				f = fakeClient.WithObjects(pod)
			}
			fClient := f.Build()

			sut := DaemonSetProber{fClient}
			ready, err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})

			require.NoError(t, err)
			require.Equal(t, tc.expected, ready)
		})
	}
}

func createPodList(ready int32, labels map[string]string) []*corev1.Pod {
	var pods []*corev1.Pod
	for i := 0; i < int(ready); i++ {
		name := fmt.Sprintf("pod-%d", i)
		pod := testutils.NewPodBuilder(name, "telemetry-system").WithLabels(labels).Build()
		pods = append(pods, pod)
	}
	return pods
}
