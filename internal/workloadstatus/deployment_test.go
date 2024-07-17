package workloadstatus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestDeploymentProber_WithStaticErrors(t *testing.T) {
	tests := []struct {
		summary          string
		desiredScheduled *int32
		numberReady      int32
		expected         bool

		pods []corev1.Pod

		expectedError error
	}{
		{
			summary:          "all scheduled all ready",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      2,
			expected:         true,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
			},
		},
		{
			summary:          "all scheduled one ready, OOM: 1 with expired threshold",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      1,
			expected:         false,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithOOMStatus().WithExpiredThreshold().Build(),
			},
			expectedError: ErrOOMKilled,
		},
		{
			summary:          "all scheduled zero ready crashbacklook: 1, OOM: 1 with expired threshold",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      0,
			expected:         false,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithCrashBackOffStatus().WithExpiredThreshold().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithOOMStatus().Build(),
			},
			expectedError: ErrContainerCrashLoop,
		},
		{
			summary:          "all scheduled zero ready but no problem",
			numberReady:      0,
			desiredScheduled: ptr.To(int32(2)),
			expected:         true,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
			},
		},
	}
	for _, test := range tests {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "telemetry-system"},
			Spec: appsv1.DeploymentSpec{
				Replicas: test.desiredScheduled,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}},
			},
			Status: appsv1.DeploymentStatus{
				// Todo: Check
				//ReadyReplicas:   test.numberReady,
				UpdatedReplicas: test.numberReady,
			},
		}

		podList := &corev1.PodList{
			Items: test.pods,
		}

		fakeClient := fake.NewClientBuilder().WithObjects(deployment).WithLists(podList).Build()
		sut := DeploymentProber{fakeClient}
		ready, err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
		require.Equal(t, test.expected, ready)
		if test.expectedError != nil {
			require.EqualError(t, err, test.expectedError.Error())
		} else {
			require.NoError(t, err)
		}
	}
}
func TestDeployment_WithErrorAssert_WithExpiredThreshold(t *testing.T) {

	tests := []struct {
		summary          string
		desiredScheduled *int32
		numberReady      int32

		pods []corev1.Pod

		expected      bool
		expectedError func(error) bool
	}{
		{
			summary:          "all scheduled 1 ready 1 evicted",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      1,
			expected:         false,
			expectedError:    isPodIsEvictedError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithEvictedStatus().Build(),
			},
		},
		{
			summary:          "all scheduled 1 ready 1 pending",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      1,
			expected:         false,
			expectedError:    isPodIsPendingError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().WithExpiredThreshold().Build(),
			},
		},
		{
			summary:          "all scheduled 1 ready 1 container not ready",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      1,
			expected:         false,
			expectedError:    isContainerNotRunningError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithImageNotFound().Build(),
			},
		},
		{
			summary:          "all scheduled 1 ready 1 process exited",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      1,
			expected:         false,
			expectedError:    isProcessInContainerExitedError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithNonZeroExitStatus().WithExpiredThreshold().Build(),
			},
		},
	}
	for _, test := range tests {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "telemetry-system"},
			Spec: appsv1.DeploymentSpec{
				Replicas: test.desiredScheduled,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}},
			},
			Status: appsv1.DeploymentStatus{
				ReadyReplicas:   test.numberReady,
				UpdatedReplicas: test.numberReady,
			},
		}

		podList := &corev1.PodList{
			Items: test.pods,
		}

		fakeClient := fake.NewClientBuilder().WithObjects(deployment).WithLists(podList).Build()
		sut := DeploymentProber{fakeClient}
		ready, err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
		require.Equal(t, test.expected, ready)
		require.Equal(t, test.expected, ready)
		require.True(t, test.expectedError(err))
	}
}

func TestDeployment_WithoutExpiredThreshold(t *testing.T) {
	tests := []struct {
		summary          string
		desiredScheduled *int32
		numberReady      int32

		pods []corev1.Pod

		expected      bool
		expectedError func(error) bool
	}{
		{
			summary:          "all scheduled one ready, OOM: 1 without expired threshold",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      1,
			expected:         true,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithOOMStatus().Build(),
			},
			expectedError: nil,
		},
		{
			summary:          "all scheduled 1 ready 1 pending without expired threshold",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      1,
			expected:         true,
			expectedError:    nil,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
			},
		},
	}
	for _, test := range tests {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "telemetry-system"},
			Spec: appsv1.DeploymentSpec{
				Replicas: test.desiredScheduled,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}},
			},
			Status: appsv1.DeploymentStatus{
				ReadyReplicas:   test.numberReady,
				UpdatedReplicas: test.numberReady,
			},
		}

		podList := &corev1.PodList{
			Items: test.pods,
		}

		fakeClient := fake.NewClientBuilder().WithObjects(deployment).WithLists(podList).Build()
		sut := DeploymentProber{fakeClient}
		ready, err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
		require.Equal(t, test.expected, ready)
		require.NoError(t, err)
	}
}

func TestDeploymentNotCreated(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	sut := DeploymentProber{fakeClient}
	ready, err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
	require.False(t, ready)
	require.NoError(t, err)
}
