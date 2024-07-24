package workloadstatus

import (
	"github.com/stretchr/testify/require"
	"testing"
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
			err:                    &ContainerNotRunningError{Message: "unable to pull image"},
			expectedErrorMsg:       "Container is not running: unable to pull image",
			expectedErrorCheckFunc: IsContainerNotRunningError,
		},
		{
			name:                   "PodIsPendingError",
			err:                    &PodIsPendingError{Message: "unable to mount volume"},
			expectedErrorMsg:       "Pod is in pending state: unable to mount volume",
			expectedErrorCheckFunc: IsPodIsPendingError,
		},
		{
			name:                   "PodIsFailedError",
			err:                    &PodIsFailedError{Message: "due to known reason"},
			expectedErrorMsg:       "Pod has failed: due to known reason",
			expectedErrorCheckFunc: IsPodFailedError,
		},
		{
			name:                   "FailedToListReplicaSetErr",
			err:                    &FailedToListReplicaSetErr{Message: "unable to list ReplicaSets"},
			expectedErrorMsg:       "failed to list ReplicaSets: unable to list ReplicaSets",
			expectedErrorCheckFunc: IsFailedToListReplicaSetErr,
		},
		{
			name:                   "FailedToFetchReplicaSetErr",
			err:                    &FailedToFetchReplicaSetErr{Message: "unable to fetch ReplicaSets"},
			expectedErrorMsg:       "failed to fetch ReplicaSets: unable to fetch ReplicaSets",
			expectedErrorCheckFunc: IsFailedToFetchReplicaSetErr,
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
