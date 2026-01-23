package gateway

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestEnrichmentValuesEmpty(t *testing.T) {
	tests := []struct {
		name               string
		labels             []string
		resourceName       types.NamespacedName
		readinessCheckFunc func(t *testing.T, name types.NamespacedName)
		genSignalType      telemetrygen.SignalType
	}{

		{
			name:               suite.LabelLogGateway,
			labels:             []string{suite.LabelLogGateway},
			resourceName:       kitkyma.LogGatewayName,
			readinessCheckFunc: assert.DeploymentReady,
			genSignalType:      telemetrygen.SignalTypeLogs,
		},
		{
			name:               fmt.Sprintf("%s-%s", suite.LabelLogGateway, suite.LabelExperimental),
			labels:             []string{suite.LabelLogGateway, suite.LabelExperimental},
			resourceName:       kitkyma.TelemetryOTLPGatewayName,
			readinessCheckFunc: assert.DaemonSetReady,
			genSignalType:      telemetrygen.SignalTypeCentralLogs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.labels...)

			var (
				uniquePrefix = unique.Prefix(tc.name)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithIncludeNamespaces(genNs).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
				Build()

			// All attributes in the enrichment flow are set to empty values
			generator := telemetrygen.NewPod(
				genNs,
				tc.genSignalType,
				telemetrygen.WithResourceAttribute("cloud.availability_zone", ""),
				telemetrygen.WithResourceAttribute("cloud.provider", ""),
				telemetrygen.WithResourceAttribute("cloud.region", ""),
				telemetrygen.WithResourceAttribute("host.arch", ""),
				telemetrygen.WithResourceAttribute("host.type", ""),
				telemetrygen.WithResourceAttribute("k8s.cluster.name", ""),
				telemetrygen.WithResourceAttribute("k8s.cluster.uid", ""),
				telemetrygen.WithResourceAttribute("k8s.cronjob.name", ""),
				telemetrygen.WithResourceAttribute("k8s.daemonset.name", ""),
				telemetrygen.WithResourceAttribute("k8s.deployment.name", ""),
				telemetrygen.WithResourceAttribute("k8s.job.name", ""),
				telemetrygen.WithResourceAttribute("k8s.namespace.name", ""),
				telemetrygen.WithResourceAttribute("k8s.node.name", ""),
				telemetrygen.WithResourceAttribute("k8s.pod.name", ""),
				telemetrygen.WithResourceAttribute("k8s.statefulset.name", ""),
				telemetrygen.WithResourceAttribute("kyma.app_name", ""),
				telemetrygen.WithResourceAttribute("kyma.input.name", ""),
				telemetrygen.WithResourceAttribute("kyma.kubernetes_io_app_name", ""),
				telemetrygen.WithResourceAttribute("service.name", ""),
			)

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
				generator.K8sObject(),
			}
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)

			tc.readinessCheckFunc(t, tc.resourceName)

			assert.OTelLogPipelineHealthy(t, pipelineName)
			assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)

			// These attributes should be enriched by the processors
			assert.BackendDataEventuallyMatches(t, backend,
				HaveFlatLogs(ContainElement(SatisfyAll(
					HaveResourceAttributes(HaveKeyWithValue("cloud.availability_zone", Not(BeEmpty()))),
					HaveResourceAttributes(HaveKeyWithValue("cloud.provider", Not(BeEmpty()))),
					HaveResourceAttributes(HaveKeyWithValue("cloud.region", Not(BeEmpty()))),
					HaveResourceAttributes(HaveKeyWithValue("host.arch", Not(BeEmpty()))),
					HaveResourceAttributes(HaveKeyWithValue("host.type", Not(BeEmpty()))),
					HaveResourceAttributes(HaveKeyWithValue("k8s.cluster.name", Not(BeEmpty()))),
					HaveResourceAttributes(HaveKeyWithValue("k8s.cluster.uid", Not(BeEmpty()))),
					HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", Not(BeEmpty()))),
					HaveResourceAttributes(HaveKeyWithValue("k8s.node.name", Not(BeEmpty()))),
					HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", Not(BeEmpty()))),
					HaveResourceAttributes(HaveKeyWithValue("service.name", Not(BeEmpty()))),
				))),
			)

			// These attributes can't be so they shouldn't be enriched by the processors (if set to empty value)
			assert.BackendDataEventuallyMatches(t, backend,
				HaveFlatLogs(ContainElement(SatisfyAll(
					HaveResourceAttributes(HaveKeyWithValue("k8s.cronjob.name", BeEmpty())),
					HaveResourceAttributes(HaveKeyWithValue("k8s.daemonset.name", BeEmpty())),
					HaveResourceAttributes(HaveKeyWithValue("k8s.deployment.name", BeEmpty())),
					HaveResourceAttributes(HaveKeyWithValue("k8s.job.name", BeEmpty())),
					HaveResourceAttributes(HaveKeyWithValue("k8s.statefulset.name", BeEmpty())),
				))),
			)

			// These attributes should be dropped by the processors
			assert.BackendDataConsistentlyMatches(t, backend,
				Not(HaveFlatLogs(ContainElement(SatisfyAny(
					HaveResourceAttributes(HaveKey("kyma.app_name")),
					HaveResourceAttributes(HaveKey("kyma.input.name")),
					HaveResourceAttributes(HaveKey("kyma.kubernetes_io_app_name")),
				)))),
			)
		})
	}
}
