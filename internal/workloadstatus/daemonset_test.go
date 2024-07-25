package workloadstatus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestDaemonSetProber_WithStaticErrors(t *testing.T) {
	tests := []struct {
		summary            string
		updatedScheduled   int32
		desiredScheduled   int32
		numberReady        int32
		observedGeneration int64
		desiredGeneration  int64

		pods []corev1.Pod

		expectedError error
	}{
		{
			summary:          "all scheduled all ready",
			desiredScheduled: 3,
			numberReady:      3,
			updatedScheduled: 3,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
			},
		},

		{
			summary:          "all scheduled one ready others have no problem",
			desiredScheduled: 3,
			numberReady:      1,
			updatedScheduled: 2,
			expectedError:    nil,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
			},
		},

		{
			summary:          "all scheduled ready:0 with no problem",
			desiredScheduled: 3,
			numberReady:      0,
			updatedScheduled: 3,
			expectedError:    nil,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
			},
		},
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

			podList := &corev1.PodList{
				Items: tc.pods,
			}

			fakeClient := fake.NewClientBuilder().WithObjects(daemonSet).WithLists(podList).Build()

			sut := DaemonSetProber{fakeClient}
			err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.NoError(t, tc.expectedError)
			}
		})
	}
}

func TestDaemonSet_WithErrorAssert(t *testing.T) {

	tests := []struct {
		summary            string
		updatedScheduled   int32
		desiredScheduled   int32
		numberReady        int32
		observedGeneration int64
		desiredGeneration  int64

		pods []corev1.Pod

		expected      bool
		expectedError func(error) bool
	}{
		{
			summary:          "all scheduled 1 ready 1 evicted",
			desiredScheduled: 3,
			numberReady:      1,
			updatedScheduled: 2,
			expected:         false,
			expectedError:    IsPodFailedError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithEvictedStatus().Build(),
			},
		},
		{
			summary:          "all scheduled ready: 0, OOM: 1, Pending:1,Crashbackloop: 1 with expired threshold",
			desiredScheduled: 3,
			numberReady:      0,
			updatedScheduled: 3,
			expected:         false,
			expectedError:    IsPodIsPendingError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithCrashBackOffStatus().Build(),
			},
		},
		{
			summary:          "all scheduled one ready one OOM and one Pending",
			desiredScheduled: 3,
			numberReady:      1,
			updatedScheduled: 3,
			expected:         false,
			expectedError:    IsPodIsPendingError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithImageNotFound().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
			},
		},
		{
			summary:          "all scheduled one ready one OOM and one Pending",
			desiredScheduled: 3,
			numberReady:      1,
			updatedScheduled: 3,
			expected:         false,
			expectedError:    IsContainerNotRunningError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithNonZeroExitStatus().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
			},
		},
		{
			summary:          "all scheduled 1 ready 1 crashbackloop with expired threshold",
			desiredScheduled: 3,
			numberReady:      1,
			updatedScheduled: 2,
			expectedError:    IsContainerNotRunningError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithCrashBackOffStatus().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
			},
		},

		{
			summary:          "all scheduled 1 ready 1 OOM with expired threshold",
			desiredScheduled: 3,
			numberReady:      1,
			updatedScheduled: 3,
			expectedError:    IsContainerNotRunningError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithOOMStatus().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithCrashBackOffStatus().Build(),
			},
		},
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

			podList := &corev1.PodList{
				Items: tc.pods,
			}

			fakeClient := fake.NewClientBuilder().WithObjects(daemonSet).WithLists(podList).Build()

			sut := DaemonSetProber{fakeClient}
			err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
			require.True(t, tc.expectedError(err))

		})
	}
}
func TestDaemonSetNotCreated(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	sut := DaemonSetProber{fakeClient}
	err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})

	require.Equal(t, ErrDaemonSetNotFound, err)
}

func TestDaemonsSetRollout(t *testing.T) {
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "telemetry-system", Generation: 1},
		Spec: appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "foo"},
		}},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 2,
			NumberReady:            1,
			UpdatedNumberScheduled: 1,
		},
	}

	pods := []corev1.Pod{
		testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
	}

	rollingOutPod := testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).Build()

	containerStatus := []corev1.ContainerStatus{{
		Name: "collector",
	}}
	rollingOutPod.Status.ContainerStatuses = containerStatus
	rollingOutPod.Status.Phase = corev1.PodPending

	pods = append(pods, rollingOutPod)
	podList := &corev1.PodList{
		Items: pods,
	}

	fakeClient := fake.NewClientBuilder().WithObjects(daemonSet).WithLists(podList).Build()
	sut := DaemonSetProber{fakeClient}
	err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
	require.Equal(t, ErrRolloutInProgress, err)
}
