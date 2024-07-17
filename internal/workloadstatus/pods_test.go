package workloadstatus

import (
	"context"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

func TestExceededTimeThreshold(t *testing.T) {
	tt := []struct {
		name       string
		timePassed metav1.Time
		expected   bool
	}{
		{
			name:       "Time passed is less than threshold",
			timePassed: metav1.Now(),
			expected:   false,
		},
		{
			name:       "Time passed is greater than threshold",
			timePassed: metav1.NewTime(time.Now().Add(-6 * time.Minute)),
			expected:   true,
		},
	}
	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.expected, exceededTimeThreshold(test.timePassed))
		})
	}
}

func TestPodStatusWithExpiredThreshold(t *testing.T) {
	tt := []struct {
		name                   string
		pod                    corev1.Pod
		expectedError          error
		expectedErrorCheckFunc func(err error) bool
	}{
		{
			name:                   "Pod is pending",
			pod:                    testutils.NewPodBuilder("foo", "default").WithPendingStatus().WithExpiredThreshold().Build(),
			expectedErrorCheckFunc: isPodIsPendingError,
		},
		{
			name:          "Invalid configuration",
			pod:           testutils.NewPodBuilder("foo", "default").WithExpiredThreshold().WithCrashBackOffStatus().Build(),
			expectedError: ErrContainerCrashLoop,
		},
		{
			name:          "container is OOMKilled",
			pod:           testutils.NewPodBuilder("foo", "default").WithExpiredThreshold().WithOOMStatus().Build(),
			expectedError: ErrOOMKilled,
		},
		{
			name:                   "process in container exited with non zero error",
			pod:                    testutils.NewPodBuilder("foo", "default").WithExpiredThreshold().WithNonZeroExitStatus().Build(),
			expectedErrorCheckFunc: isProcessInContainerExitedError,
		},
		{
			name:                   "Pod is evicted",
			pod:                    testutils.NewPodBuilder("foo", "default").WithEvictedStatus().WithExpiredThreshold().Build(),
			expectedErrorCheckFunc: isPodIsEvictedError,
		},
		{
			name:          "Pod is running",
			pod:           testutils.NewPodBuilder("foo", "default").WithRunningStatus().Build(),
			expectedError: nil,
		},
		{
			name:                   "Pod cannot pull image",
			pod:                    testutils.NewPodBuilder("foo", "default").WithImageNotFound().Build(),
			expectedErrorCheckFunc: isContainerNotRunningError,
		},
	}
	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			//t.Parallel()
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
