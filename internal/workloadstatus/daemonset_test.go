package workloadstatus

import (
	"context"
	"errors"
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

		expected      bool
		expectedError error
	}{
		{
			summary:          "all scheduled all ready",
			desiredScheduled: 3,
			numberReady:      3,
			updatedScheduled: 3,
			expected:         true,
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
			expected:         true,
			expectedError:    nil,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
			},
		},
		{
			summary:          "all scheduled 1 ready 1 crashbackloop with expired threshold",
			desiredScheduled: 3,
			numberReady:      1,
			updatedScheduled: 2,
			expected:         false,
			expectedError:    ErrContainerCrashLoop,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithCrashBackOffStatus().WithExpiredThreshold().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().WithExpiredThreshold().Build(),
			},
		},

		{
			summary:          "all scheduled 1 ready 1 OOM with expired threshold",
			desiredScheduled: 3,
			numberReady:      1,
			updatedScheduled: 3,
			expected:         false,
			expectedError:    ErrOOMKilled,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithOOMStatus().WithExpiredThreshold().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithCrashBackOffStatus().WithExpiredThreshold().Build(),
			},
		},

		{
			summary:          "all scheduled ready:0 with no problem",
			desiredScheduled: 3,
			numberReady:      0,
			updatedScheduled: 3,
			expected:         true,
			expectedError:    nil,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
			},
		},

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

			podList := &corev1.PodList{
				Items: tc.pods,
			}

			fakeClient := fake.NewClientBuilder().WithObjects(daemonSet).WithLists(podList).Build()

			sut := DaemonSetProber{fakeClient}
			ready, _ := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
			require.Equal(t, tc.expected, ready)
			if tc.expectedError != nil {
				require.EqualError(t, tc.expectedError, tc.expectedError.Error())
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
			expectedError:    isPodIsEvictedError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithEvictedStatus().WithExpiredThreshold().Build(),
			},
		},
		{
			summary:          "all scheduled ready: 0, OOM: 1, Pending:1,Crashbackloop: 1 with expired threshold",
			desiredScheduled: 3,
			numberReady:      0,
			updatedScheduled: 3,
			expected:         false,
			expectedError:    isPodIsPendingError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().WithExpiredThreshold().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().WithExpiredThreshold().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithCrashBackOffStatus().WithExpiredThreshold().Build(),
			},
		},
		{
			summary:          "all scheduled one ready one OOM and one Pending",
			desiredScheduled: 3,
			numberReady:      1,
			updatedScheduled: 3,
			expected:         false,
			expectedError:    isContainerNotRunningError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithImageNotFound().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().WithExpiredThreshold().Build(),
			},
		},
		{
			summary:          "all scheduled one ready one OOM and one Pending",
			desiredScheduled: 3,
			numberReady:      1,
			updatedScheduled: 3,
			expected:         false,
			expectedError:    isProcessInContainerExitedError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithExpiredThreshold().WithNonZeroExitStatus().Build(),
				testutils.NewPodBuilder("pod-2", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().WithExpiredThreshold().Build(),
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
			ready, err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
			require.Equal(t, tc.expected, ready)
			require.True(t, tc.expectedError(err))

		})
	}
}
func TestDaemonSetNotCreated(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	sut := DaemonSetProber{fakeClient}
	ready, err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
	require.False(t, ready)
	require.NoError(t, err)
}

func TestDaemonSetError(t *testing.T) {
	err := &DaemonSetFetchingError{
		Name:      "foo",
		Namespace: "telemetry-system",
		Err:       errors.New("unable to find daemonset due to unknown reason"),
	}
	require.EqualError(t, err, "failed to get telemetry-system/foo DaemonSet: unable to find daemonset due to unknown reason")
	require.True(t, IsDaemonSetFetchingError(err))
}
