package workloadstatus

import (
	"errors"
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
			name:                   "Unable to pull image",
			err:                    &PodIsPendingError{ContainerName: "foo", Reason: "ErrImagePull", Message: "unable to pull image"},
			expectedErrorMsg:       "Pod is in pending state: reason: ErrImagePull, message: unable to pull image",
			expectedErrorCheckFunc: IsPodIsPendingError,
		},
		{
			name:                   "PodIsPendingError",
			err:                    &PodIsNotScheduledError{Message: "unable to mount volume"},
			expectedErrorMsg:       "Pod is not scheduled: unable to mount volume",
			expectedErrorCheckFunc: IsPodIsNotScheduledError,
		},
		{
			name:                   "PodIsFailingError",
			err:                    &PodIsFailingError{Message: "due to known reason"},
			expectedErrorMsg:       "Pod has failed: due to known reason",
			expectedErrorCheckFunc: IsPodFailedError,
		},
		{
			name:                   "FailedToListReplicaSetError",
			err:                    &FailedToListReplicaSetError{ErrorObj: errors.New("unable to list ReplicaSets")},
			expectedErrorMsg:       "failed to list ReplicaSets: unable to list ReplicaSets",
			expectedErrorCheckFunc: IsFailedToListReplicaSetErr,
		},
		{
			name:                   "FailedToFetchReplicaSetError",
			err:                    &FailedToFetchReplicaSetError{ErroObj: errors.New("unable to fetch ReplicaSets")},
			expectedErrorMsg:       "failed to fetch ReplicaSets: unable to fetch ReplicaSets",
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
