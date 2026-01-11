package traces

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestTransform(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

	tests := []struct {
		name          string
		transformSpec telemetryv1beta1.TransformSpec
		assertion     types.GomegaMatcher
		tranceGen     func(ns string) client.Object
	}{
		{
			name: "with-where",
			tranceGen: func(ns string) client.Object {
				return telemetrygen.NewPod(ns, telemetrygen.SignalTypeTraces).K8sObject()
			},
			transformSpec: telemetryv1beta1.TransformSpec{
				Statements: []string{"set(span.attributes[\"system\"], \"false\") where not IsMatch(resource.attributes[\"k8s.namespace.name\"], \".*-system\")"},
			},
			assertion: HaveFlatTraces(ContainElement(SatisfyAll(
				HaveResourceAttributes(Not(HaveKeyWithValue("k8s.namespace.name", "kyma-system"))),
				HaveSpanAttributes(HaveKeyWithValue("system", "false")),
			))),
		}, {
			name: "cond-and-stmts",
			tranceGen: func(ns string) client.Object {
				return telemetrygen.NewPod(ns, telemetrygen.SignalTypeTraces, telemetrygen.WithTelemetryAttribute("component", "proxy")).K8sObject()
			},
			transformSpec: telemetryv1beta1.TransformSpec{
				Conditions: []string{"span.attributes[\"component\"] == \"proxy\""},
				Statements: []string{"set(span.attributes[\"FromProxy\"], \"true\")"},
			},
			assertion: HaveFlatTraces(ContainElement(SatisfyAll(
				HaveSpanAttributes(HaveKeyWithValue("component", "proxy")),
				HaveSpanAttributes(HaveKeyWithValue("FromProxy", "true")),
			))),
		}, {
			name: "infer-context",
			tranceGen: func(ns string) client.Object {
				return telemetrygen.NewPod(ns, telemetrygen.SignalTypeTraces).K8sObject()
			},
			transformSpec: telemetryv1beta1.TransformSpec{
				Statements: []string{"set(span.attributes[\"name\"], \"passed\")",
					"set(resource.attributes[\"test\"], \"passed\")",
				},
			},
			assertion: HaveFlatTraces(ContainElement(SatisfyAll(
				HaveSpanAttributes(HaveKeyWithValue("name", "passed")),
				HaveResourceAttributes(HaveKeyWithValue("test", "passed")),
			))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				uniquePrefix = unique.Prefix("traces", tt.name)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)
			pipeline := testutils.NewTracePipelineBuilder().
				WithName(pipelineName).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
				WithTransform(tt.transformSpec).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
				tt.tranceGen(genNs),
			}
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.TraceGatewayName)
			assert.TracePipelineHealthy(t, pipelineName)

			assert.BackendDataEventuallyMatches(t, backend, tt.assertion)
		})
	}
}
