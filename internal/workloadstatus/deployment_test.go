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

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestDeploymentProber_WithStaticErrors(t *testing.T) {
	tests := []struct {
		summary          string
		desiredScheduled *int32
		numberReady      int32
		pods             []corev1.Pod
		expectedError    error
	}{
		{
			summary:          "all scheduled, all ready",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      2,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.summary, func(t *testing.T) {
			t.Parallel()

			deployment := createDeployment(test.desiredScheduled, test.numberReady)
			replicaSet := createReplicaSet(test.desiredScheduled, test.numberReady, *deployment)

			itemList := make([]appsv1.ReplicaSet, 0)

			itemList = append(itemList, *replicaSet)
			rsList := &appsv1.ReplicaSetList{
				Items: itemList,
			}

			podList := &corev1.PodList{
				Items: test.pods,
			}

			fakeClient := fake.NewClientBuilder().WithObjects(deployment).WithLists(rsList, podList).Build()
			sut := DeploymentProber{fakeClient}

			err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
			if test.expectedError != nil {
				require.EqualError(t, err, test.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDeployment_WithErrorAssert(t *testing.T) {
	tests := []struct {
		summary          string
		desiredScheduled *int32
		numberReady      int32
		pods             []corev1.Pod
		expectedError    func(error) bool
	}{
		{
			summary:          "all scheduled, zero ready but no problem",
			numberReady:      0,
			desiredScheduled: ptr.To(int32(2)),
			expectedError:    IsPodIsPendingError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
			},
		},
		{
			summary:          "all scheduled, 1 ready, 1 evicted",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      1,
			expectedError:    IsPodFailedError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithEvictedStatus().Build(),
			},
		},
		{
			summary:          "all scheduled, 1 ready, 1 pending",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      1,
			expectedError:    IsPodIsPendingError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithPendingStatus().Build(),
			},
		},
		{
			summary:          "all scheduled, 1 ready, 1 container not ready",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      1,
			expectedError:    IsPodIsPendingError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithImageNotFound().Build(),
			},
		},
		{
			summary:          "all scheduled, 1 ready, 1 process exited",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      1,
			expectedError:    IsPodIsPendingError,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithNonZeroExitStatus().Build(),
			},
		},
		{
			summary:          "all scheduled one ready, OOM: 1 with expired threshold",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      1,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithRunningStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithOOMStatus().Build(),
			},
			expectedError: IsPodIsPendingError,
		},
		{
			summary:          "all scheduled, 0 ready, crashloop: 1, OOM: 1 with expired threshold",
			desiredScheduled: ptr.To(int32(2)),
			numberReady:      0,
			pods: []corev1.Pod{
				testutils.NewPodBuilder("pod-0", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithCrashBackOffStatus().Build(),
				testutils.NewPodBuilder("pod-1", "telemetry-system").WithLabels(map[string]string{"app": "foo"}).WithOOMStatus().Build(),
			},
			expectedError: IsPodIsPendingError,
		},
	}
	for _, test := range tests {
		t.Run(test.summary, func(t *testing.T) {
			t.Parallel()

			deployment := createDeployment(test.desiredScheduled, test.numberReady)
			replicaSet := createReplicaSet(test.desiredScheduled, test.numberReady, *deployment)

			itemList := make([]appsv1.ReplicaSet, 0)

			itemList = append(itemList, *replicaSet)
			rsList := &appsv1.ReplicaSetList{
				Items: itemList,
			}

			podList := &corev1.PodList{
				Items: test.pods,
			}

			fakeClient := fake.NewClientBuilder().WithObjects(deployment).WithLists(rsList, podList).Build()
			sut := DeploymentProber{fakeClient}
			err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})

			require.True(t, test.expectedError(err))
		})
	}
}

func TestDeploymentNotCreated(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	sut := DeploymentProber{fakeClient}
	err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
	require.Equal(t, ErrDeploymentNotFound, err)
}

func createDeployment(desiredScheduled *int32, numberReady int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "telemetry-system"},
		Spec: appsv1.DeploymentSpec{
			Replicas: desiredScheduled,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}},
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: numberReady,
		},
	}
}

func createReplicaSet(desiredScheduled *int32, numberReady int32, dep appsv1.Deployment) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Namespace:       "telemetry-system",
			Labels:          dep.Spec.Selector.MatchLabels,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&dep, dep.GroupVersionKind())},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: dep.Spec.Selector,
			Replicas: desiredScheduled,
			Template: dep.Spec.Template,
		},
		Status: appsv1.ReplicaSetStatus{
			ReadyReplicas: numberReady,
			Replicas:      numberReady,
		},
	}
}

func TestDeploymentSetRollout(t *testing.T) {
	deployment := createDeployment(ptr.To(int32(2)), 1)
	replicaSet := createReplicaSet(ptr.To(int32(2)), 1, *deployment)

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

	replicaSets := make([]appsv1.ReplicaSet, 0)
	replicaSets = append(replicaSets, *replicaSet)
	rsList := &appsv1.ReplicaSetList{
		Items: replicaSets,
	}

	fakeClient := fake.NewClientBuilder().WithObjects(deployment).WithLists(rsList, podList).Build()
	sut := DeploymentProber{fakeClient}
	err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
	require.True(t, IsRolloutInProgressError(err))
}

func TestReplicaSetNotFound(t *testing.T) {
	deployment := createDeployment(ptr.To(int32(2)), 1)

	fakeClient := fake.NewClientBuilder().WithObjects(deployment).Build()
	sut := DeploymentProber{fakeClient}
	err := sut.IsReady(context.Background(), types.NamespacedName{Name: "foo", Namespace: "telemetry-system"})
	require.Equal(t, err, ErrFailedToGetLatestReplicaSet)
}
