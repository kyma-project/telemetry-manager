package misc

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestManager(t *testing.T) {
	suite.SetupTest(t, suite.LabelTelemetry)

	assert.DeploymentReady(t, types.NamespacedName{
		Name:      "telemetry-manager",
		Namespace: kitkyma.SystemNamespaceName})

	resources := []assert.Resource{
		assert.NewResource(&corev1.Namespace{}, types.NamespacedName{Name: kitkyma.SystemNamespaceName}),
		assert.NewResource(&corev1.Service{}, kitkyma.TelemetryManagerWebhookServiceName),
		assert.NewResource(&corev1.Service{}, kitkyma.TelemetryManagerMetricsServiceName),
		assert.NewResource(&apiextensionsv1.CustomResourceDefinition{}, types.NamespacedName{Name: "logpipelines.telemetry.kyma-project.io"}),
		assert.NewResource(&apiextensionsv1.CustomResourceDefinition{}, types.NamespacedName{Name: "tracepipelines.telemetry.kyma-project.io"}),
		assert.NewResource(&apiextensionsv1.CustomResourceDefinition{}, types.NamespacedName{Name: "metricpipelines.telemetry.kyma-project.io"}),
		assert.NewResource(&apiextensionsv1.CustomResourceDefinition{}, types.NamespacedName{Name: "telemetries.operator.kyma-project.io"}),
		assert.NewResource(&corev1.ConfigMap{}, types.NamespacedName{Name: "telemetry-metricpipelines", Namespace: kitkyma.SystemNamespaceName}),
		assert.NewResource(&corev1.ConfigMap{}, types.NamespacedName{Name: "telemetry-logpipelines", Namespace: kitkyma.SystemNamespaceName}),
		assert.NewResource(&corev1.ConfigMap{}, types.NamespacedName{Name: "telemetry-tracepipelines", Namespace: kitkyma.SystemNamespaceName}),
		assert.NewResource(&corev1.ConfigMap{}, types.NamespacedName{Name: "telemetry-module", Namespace: kitkyma.SystemNamespaceName}),
		assert.NewResource(&networkingv1.NetworkPolicy{}, types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + "telemetry-manager", Namespace: kitkyma.SystemNamespaceName}),
		assert.NewResource(&schedulingv1.PriorityClass{}, types.NamespacedName{Name: "telemetry-priority-class", Namespace: kitkyma.SystemNamespaceName}),
		assert.NewResource(&schedulingv1.PriorityClass{}, types.NamespacedName{Name: "telemetry-priority-class-high", Namespace: kitkyma.SystemNamespaceName}),
	}

	assert.ResourcesExist(t, resources...)

	services := []types.NamespacedName{kitkyma.TelemetryManagerWebhookServiceName, kitkyma.TelemetryManagerMetricsServiceName}
	for _, service := range services {
		Eventually(func() []string {
			var svc corev1.Service

			err := suite.K8sClient.Get(suite.Ctx, service, &svc)
			Expect(err).NotTo(HaveOccurred())

			if service == kitkyma.TelemetryManagerMetricsServiceName {
				Expect(svc.Annotations).Should(HaveKeyWithValue("prometheus.io/scrape", "true"))
				Expect(svc.Annotations).Should(HaveKeyWithValue("prometheus.io/port", "8080"))
			}

			var endpointsList discoveryv1.EndpointSliceList

			err = suite.K8sClient.List(suite.Ctx, &endpointsList, client.InNamespace(kitkyma.SystemNamespaceName))
			Expect(err).NotTo(HaveOccurred())

			var webhookEndpoints *discoveryv1.EndpointSlice

			for _, endpoints := range endpointsList.Items {
				// EndpointSlice names are prefixed with the service name
				if strings.HasPrefix(endpoints.Name, service.Name) {
					webhookEndpoints = &endpoints
					break
				}
			}

			Expect(webhookEndpoints).NotTo(BeNil())

			var addresses []string
			for _, endpoint := range webhookEndpoints.Endpoints {
				addresses = append(addresses, endpoint.Addresses...)
			}

			return addresses
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(BeEmpty(), fmt.Sprintf("service %s endpoints should have IP addresses assigned", service.Name))
	}
}
