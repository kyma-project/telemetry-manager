package assert

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func JobReady(ctx context.Context, name types.NamespacedName) {
	Eventually(func(g Gomega) {
		ready, err := isJobSuccessful(ctx, suite.K8sClient, name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrueBecause("Job not ready: %s", name.String()))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func isJobSuccessful(ctx context.Context, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	var job batchv1.Job

	err := k8sClient.Get(ctx, name, &job)
	if err != nil {
		return false, fmt.Errorf("failed to get Job %s: %w", name.String(), err)
	}

	return job.Status.Active > 0, nil
}
