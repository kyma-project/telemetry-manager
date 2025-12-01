package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestTransform_OTel(t *testing.T) {
	tests := []struct {
		label               string
		name                string
		input               telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
		transformSpec       telemetryv1alpha1.TransformSpec
		assertion           types.GomegaMatcher
		expectAgent         bool
	}{
		{
			label: suite.LabelLogAgent,
			name:  "with-where",
			input: testutils.BuildLogPipelineApplicationInput(),
			logGeneratorBuilder: func(ns string) client.Object {
				return stdoutloggen.NewDeployment(ns).K8sObject()
			},
			transformSpec: telemetryv1alpha1.TransformSpec{
				Statements: []string{"set(log.attributes[\"system\"], \"false\") where not IsMatch(resource.attributes[\"k8s.namespace.name\"], \".*-system\")"},
			},
			assertion: HaveFlatLogs(ContainElement(SatisfyAll(
				HaveResourceAttributes(Not(HaveKeyWithValue("k8s.namespace.name", "kyma-system"))),
				HaveAttributes(HaveKeyWithValue("system", "false")),
			))),
			expectAgent: true,
		},
		{
			label: suite.LabelLogAgent,
			name:  "infer-context",
			input: testutils.BuildLogPipelineApplicationInput(),
			logGeneratorBuilder: func(ns string) client.Object {
				return stdoutloggen.NewDeployment(ns, stdoutloggen.WithFields(map[string]string{
					"scenario": "level-info",
					"level":    "info",
				})).K8sObject()
			},
			expectAgent: true,
			transformSpec: telemetryv1alpha1.TransformSpec{
				Statements: []string{"set(resource.attributes[\"test\"], \"passed\")",
					"set(log.attributes[\"name\"], \"InfoLogs\")",
				},
			},
			assertion: HaveFlatLogs(ContainElement(SatisfyAll(
				HaveResourceAttributes(HaveKeyWithValue("test", "passed")),
				HaveAttributes(HaveKeyWithValue("name", "InfoLogs")),
			))),
		}, {
			label: suite.LabelLogAgent,
			name:  "cond-and-stmts",
			input: testutils.BuildLogPipelineApplicationInput(),
			logGeneratorBuilder: func(ns string) client.Object {
				return stdoutloggen.NewDeployment(ns, stdoutloggen.WithFields(map[string]string{
					"scenario": "level-info",
					"level":    "info",
				})).K8sObject()
			},
			expectAgent: true,
			transformSpec: telemetryv1alpha1.TransformSpec{
				Conditions: []string{"log.severity_text == \"info\" or log.severity_text == \"Info\""},
				Statements: []string{"set(log.severity_text, ToUpperCase(log.severity_text))"},
			},
			assertion: HaveFlatLogs(ContainElement(SatisfyAll(
				HaveSeverityText(Equal("INFO")),
			))),
		},
		{
			label: suite.LabelLogGateway,
			name:  "with-where",
			input: testutils.BuildLogPipelineOTLPInput(),
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
			transformSpec: telemetryv1alpha1.TransformSpec{
				Statements: []string{"set(log.attributes[\"system\"], \"false\") where not IsMatch(resource.attributes[\"k8s.namespace.name\"], \".*-system\")"},
			},
			assertion: HaveFlatLogs(ContainElement(SatisfyAll(
				HaveResourceAttributes(Not(HaveKeyWithValue("k8s.namespace.name", "kyma-system"))),
				HaveAttributes(HaveKeyWithValue("system", "false")),
			))),
		}, {
			label: suite.LabelLogGateway,
			name:  "infer-context",
			input: testutils.BuildLogPipelineOTLPInput(),
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
			transformSpec: telemetryv1alpha1.TransformSpec{
				Statements: []string{"set(resource.attributes[\"test\"], \"passed\")",
					"set(log.attributes[\"name\"], \"InfoLogs\")",
				},
			},
			assertion: HaveFlatLogs(ContainElement(SatisfyAll(
				HaveResourceAttributes(HaveKeyWithValue("test", "passed")),
				HaveAttributes(HaveKeyWithValue("name", "InfoLogs")),
			))),
		}, {
			label: suite.LabelLogGateway,
			name:  "cond-and-stmts",
			input: testutils.BuildLogPipelineOTLPInput(),
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
			transformSpec: telemetryv1alpha1.TransformSpec{
				Conditions: []string{"log.severity_text == \"info\" or log.severity_text == \"Info\""},
				Statements: []string{"set(log.severity_text, ToUpperCase(log.severity_text))"},
			},
			assertion: HaveFlatLogs(ContainElement(SatisfyAll(
				HaveSeverityText(Equal("INFO")),
			))),
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			var (
				uniquePrefix      = unique.Prefix("logs", tc.name)
				pipelineNameValue = uniquePrefix("value")
				backendNs         = uniquePrefix("backend")
				genNs             = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

			pipelineTransform := testutils.NewLogPipelineBuilder().
				WithName(pipelineNameValue).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				WithTransform(tc.transformSpec).
				Build()

			resources := []client.Object{
				objects.NewNamespace(backendNs).K8sObject(),
				objects.NewNamespace(genNs).K8sObject(),
				tc.logGeneratorBuilder(genNs),
				&pipelineTransform,
			}

			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
			})

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.LogGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
			}

			assert.OTelLogPipelineHealthy(t, pipelineNameValue)

			assert.BackendDataEventuallyMatches(t, backend, tc.assertion)
		})
	}
}
