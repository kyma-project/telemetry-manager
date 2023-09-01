package verifiers

import (
	"context"

	v1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsJobCompleted(ctx context.Context, k8sClient client.Client, listOptions client.ListOptions) (bool, error) {
	var jobs v1.JobList
	err := k8sClient.List(ctx, &jobs, &listOptions)
	if err != nil {
		return false, err
	}
	for _, job := range jobs.Items {
		for _, condition := range job.Status.Conditions {
			if condition.Type == v1.JobComplete {
				return true, nil
			}
		}
	}
	return false, nil
}
