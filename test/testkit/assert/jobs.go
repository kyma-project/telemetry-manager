package assert

import (
	"fmt"

	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func JobReady(t testkit.T, name types.NamespacedName) {
	t.Helper()

	Eventually(func(g Gomega) {
		ready, err := isJobSuccessful(t, suite.K8sClient, name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrueBecause("Job not ready: %s", name.String()))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func isJobSuccessful(t testkit.T, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	t.Helper()

	var job batchv1.Job
	err := k8sClient.Get(t.Context(), name, &job)
	if err != nil {
		return false, fmt.Errorf("failed to get Job %s: %w", name.String(), err)
	}

	return job.Status.Active > 0, nil
}
