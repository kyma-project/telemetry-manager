package workloadstatus

import (
	"context"
	"errors"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestPodStatusWithExpiredThreshold(t *testing.T) {
	tt := []struct {
		name              string
		pod               *corev1.Pod
		checkUsingErrorAs bool
		expectedError     error
	}{
		{
			name:              "Pod is pending",
			pod:               testutils.NewPodBuilder("foo", "default").WithExpiredThreshold().WithPendingStatus().Build(),
			checkUsingErrorAs: true,
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
			name:              "process in container exited with non zero error",
			pod:               testutils.NewPodBuilder("foo", "default").WithExpiredThreshold().WithNonZeroExitStatus().Build(),
			checkUsingErrorAs: true,
		},
		{
			name:              "Pod is evicted",
			pod:               testutils.NewPodBuilder("foo", "default").WithEvictedStatus().WithExpiredThreshold().Build(),
			checkUsingErrorAs: true,
		},
		{
			name:          "Pod is running",
			pod:           testutils.NewPodBuilder("foo", "default").WithRunningStatus().WithExpiredThreshold().Build(),
			expectedError: nil,
		},
	}
	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fakeClient := fake.NewClientBuilder().WithObjects(test.pod).Build()

			err := checkPodStatus(context.Background(), fakeClient, "default", &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}})
			if test.checkUsingErrorAs {
				require.True(t, isPodError(err))
				var podError *PodsError
				require.True(t, errors.As(err, &podError))
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
		pod    *corev1.Pod
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
			fakeClient := fake.NewClientBuilder().WithObjects(test.pod).Build()

			err := checkPodStatus(context.Background(), fakeClient, "default", &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}})
			require.NoError(t, err)
		})
	}
}
