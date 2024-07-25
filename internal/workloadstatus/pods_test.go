package workloadstatus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestPodStatus(t *testing.T) {
	tt := []struct {
		name                   string
		pod                    corev1.Pod
		expectedError          error
		expectedErrorCheckFunc func(err error) bool
	}{
		{
			name:                   "Pod is pending",
			pod:                    testutils.NewPodBuilder("foo", "default").WithPendingStatus().Build(),
			expectedErrorCheckFunc: IsPodIsPendingError,
		},
		{
			name:                   "Invalid configuration",
			pod:                    testutils.NewPodBuilder("foo", "default").WithCrashBackOffStatus().Build(),
			expectedErrorCheckFunc: IsContainerNotRunningError,
		},
		{
			name:                   "container is OOMKilled",
			pod:                    testutils.NewPodBuilder("foo", "default").WithOOMStatus().Build(),
			expectedErrorCheckFunc: IsContainerNotRunningError,
		},
		{
			name:                   "process in container exited with non zero error",
			pod:                    testutils.NewPodBuilder("foo", "default").WithNonZeroExitStatus().Build(),
			expectedErrorCheckFunc: IsContainerNotRunningError,
		},
		{
			name:                   "Pod is evicted",
			pod:                    testutils.NewPodBuilder("foo", "default").WithEvictedStatus().Build(),
			expectedErrorCheckFunc: IsPodFailedError,
		},
		{
			name:          "Pod is running",
			pod:           testutils.NewPodBuilder("foo", "default").WithRunningStatus().Build(),
			expectedError: nil,
		},
		{
			name:                   "Pod cannot pull image",
			pod:                    testutils.NewPodBuilder("foo", "default").WithImageNotFound().Build(),
			expectedErrorCheckFunc: IsPodIsPendingError,
		},
	}
	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fakeClient := fake.NewClientBuilder().WithObjects(&test.pod).Build()

			err := checkPodStatus(context.Background(), fakeClient, "default", &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}})
			if test.expectedErrorCheckFunc != nil {
				require.True(t, test.expectedErrorCheckFunc(err))
			} else {
				require.Equal(t, test.expectedError, err)
			}
		})
	}
}

func TestNoPods(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	err := checkPodStatus(context.Background(), fakeClient, "default", &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}})
	require.Equal(t, err, ErrNoPodsDeployed)

}

func TestPodPendingStatus(t *testing.T) {
	tests := []struct {
		name              string
		pod               corev1.Pod
		expectedErrorFunc func(err error) bool
		expectedError     error
		waitingState      *corev1.ContainerStateWaiting
	}{
		{
			name:              "Pod is pending but container state is waiting is not set",
			pod:               testutils.NewPodBuilder("foo", "default").Build(),
			waitingState:      nil,
			expectedErrorFunc: nil,
			expectedError:     nil,
		},
		{
			name:              "Pod is pending but container state has no reason set",
			pod:               testutils.NewPodBuilder("foo", "default").Build(),
			waitingState:      &corev1.ContainerStateWaiting{Reason: "", Message: "foo Message"},
			expectedErrorFunc: IsPodIsPendingError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			containerStatus := []corev1.ContainerStatus{{
				Name:  "collector",
				State: corev1.ContainerState{Waiting: test.waitingState},
			}}
			test.pod.Status.ContainerStatuses = containerStatus
			test.pod.Status.Phase = corev1.PodPending
			fakeClient := fake.NewClientBuilder().WithObjects(&test.pod).Build()
			err := checkPodStatus(context.Background(), fakeClient, "default", &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}})
			if test.expectedErrorFunc != nil {
				require.True(t, test.expectedErrorFunc(err))
			} else {
				require.Equal(t, test.expectedError, err)
			}
		})
	}
}

func TestPodWaitingStatus(t *testing.T) {
	pod := testutils.NewPodBuilder("foo", "default").Build()
	containerStatus := []corev1.ContainerStatus{{
		Name: "collector",
		State: corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{
				Reason: "foo Reason",
			},
		},
	}}

	pod.Status.ContainerStatuses = containerStatus
	pod.Status.Phase = corev1.PodRunning
	fakeClient := fake.NewClientBuilder().WithObjects(&pod).Build()
	err := checkPodStatus(context.Background(), fakeClient, "default", &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}})
	require.True(t, IsContainerNotRunningError(err))

}
