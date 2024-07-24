package workloadstatus

import (
	"context"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"

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
			expectedErrorCheckFunc: IsImageNotPulledError,
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

func TestPodStatusWithoutExpiredThreshold(t *testing.T) {
	tt := []struct {
		name   string
		status corev1.PodStatus
		pod    corev1.Pod
	}{
		{
			name: "Pod is pending",
			pod:  testutils.NewPodBuilder("foo", "default").WithPendingStatus().Build(),
		},
		{
			name: "Invalid configuration",
			pod:  testutils.NewPodBuilder("foo", "default").WithCrashBackOffStatus().Build(),
		},
	}

	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fakeClient := fake.NewClientBuilder().WithObjects(&test.pod).Build()

			err := checkPodStatus(context.Background(), fakeClient, "default", &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}})
			require.NoError(t, err)
		})
	}
}

func TestErrorMessages(t *testing.T) {
	tt := []struct {
		name             string
		err              error
		expectedErrorMsg string
	}{
		{
			name:             "ContainerNotRunningError",
			err:              &ContainerNotRunningError{Message: "unable to pull image"},
			expectedErrorMsg: "Container is not running: unable to pull image",
		},
		{
			name:             "PodIsPendingError",
			err:              &PodIsPendingError{Message: "unable to mount volume"},
			expectedErrorMsg: "Pod is in pending state: unable to mount volume",
		},
		{
			name:             "PodIsFailedError",
			err:              &PodIsFailedError{Message: "due to known reason"},
			expectedErrorMsg: "Pod has failed: due to known reason",
		},
	}
	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.expectedErrorMsg, test.err.Error())
		})
	}
}

func TestNoPods(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	err := checkPodStatus(context.Background(), fakeClient, "default", &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}})
	require.Equal(t, err, ErrNoPodsDeployed)

}
