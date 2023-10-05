package verifiers

import (
	"context"
	"net/http"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/apiserver"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func LogsShouldBeDelivered(proxyClient *apiserver.ProxyClient, logProducerName string, telemetryExportURL string) {
	gomega.Eventually(func(g gomega.Gomega) {
		resp, err := proxyClient.Get(telemetryExportURL)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(gomega.HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(gomega.HaveHTTPBody(gomega.SatisfyAll(
			matchers.ContainLogs(matchers.WithPod(logProducerName)))))
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(gomega.Succeed())
}

func LogPipelineShouldBeRunning(ctx context.Context, k8sClient client.Client, pipelineName string) {
	gomega.Eventually(func(g gomega.Gomega) bool {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(gomega.Succeed())
		return pipeline.Status.HasCondition(telemetryv1alpha1.LogPipelineRunning)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(gomega.BeTrue())
}
