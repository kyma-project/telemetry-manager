package workloadstatus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorMessages(t *testing.T) {
	tt := []struct {
		name                   string
		err                    error
		expectedErrorMsg       string
		expectedErrorCheckFunc func(err error) bool
	}{
		{
			name:                   "ContainerNotRunningError",
			err:                    &PodIsPendingError{Message: "unable to pull image"},
			expectedErrorMsg:       "Pod is in pending state: unable to pull image",
			expectedErrorCheckFunc: IsPodIsPendingError,
		},
		{
			name:                   "PodIsPendingError",
			err:                    &PodIsPendingError{Message: "unable to mount volume"},
			expectedErrorMsg:       "Pod is in pending state: unable to mount volume",
			expectedErrorCheckFunc: IsPodIsPendingError,
		},
		{
			name:                   "PodIsFailingError",
			err:                    &PodIsFailingError{Message: "due to known reason"},
			expectedErrorMsg:       "Pod has failed: due to known reason",
			expectedErrorCheckFunc: IsPodFailedError,
		},
		{
			name:                   "FailedToListReplicaSetError",
			err:                    &FailedToListReplicaSetError{Message: "unable to list ReplicaSets"},
			expectedErrorMsg:       "Failed to list ReplicaSets: unable to list ReplicaSets",
			expectedErrorCheckFunc: IsFailedToListReplicaSetErr,
		},
		{
			name:                   "FailedToFetchReplicaSetError",
			err:                    &FailedToFetchReplicaSetError{Message: "unable to fetch ReplicaSets"},
			expectedErrorMsg:       "Failed to fetch ReplicaSets: unable to fetch ReplicaSets",
			expectedErrorCheckFunc: IsFailedToFetchReplicaSetError,
		},
		{
			name:                   "RolloutInProgressError",
			err:                    &RolloutInProgressError{},
			expectedErrorMsg:       "Rollout is in progress",
			expectedErrorCheckFunc: IsRolloutInProgressError,
		},
	}
	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.expectedErrorMsg, test.err.Error())
			require.True(t, test.expectedErrorCheckFunc(test.err))
		})
	}
}
