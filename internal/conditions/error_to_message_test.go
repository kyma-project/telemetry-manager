package conditions

import (
	"testing"

	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func TestErrorConverter(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "PodIsNotScheduledError",
			err:  &workloadstatus.PodIsNotScheduledError{Message: "pvc not mounted"},
			want: "Pod is not scheduled: pvc not mounted",
		},
		{
			name: "PodIsPendingError Without Reason",
			err:  &workloadstatus.PodIsPendingError{ContainerName: "fluent-bit", Message: "Error"},
			want: "Pod is in pending state as container: fluent-bit is not running due to: Error",
		},
		{
			name: "PodIsPendingError With Reason",
			err:  &workloadstatus.PodIsPendingError{ContainerName: "fluent-bit", Reason: "CrashLoopBackOff"},
			want: "Pod is in pending state as container: fluent-bit is not running due to: CrashLoopBackOff",
		},
		{
			name: "PodIsFailedError",
			err:  &workloadstatus.PodIsFailingError{Message: "Pod is evicted"},
			want: "Pod is in failed state due to: Pod is evicted",
		},
		{
			name: "RolloutInProgressError",
			err:  &workloadstatus.RolloutInProgressError{},
			want: "Pods are being started/updated",
		},
		{
			name: "Error string",
			err:  workloadstatus.ErrDeploymentFetching,
			want: "Failed to get Deployment",
		},
	}
	etc := &ErrorToMessageConverter{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := etc.Convert(tt.err)
			if got != tt.want {
				t.Errorf("ErrorToMessageConverter.Convert() = %v, want %v", got, tt.want)
			}
		})
	}
}
