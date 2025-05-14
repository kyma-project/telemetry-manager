package shared

import (
	"testing"

	. "github.com/onsi/gomega"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestSecretrefMisconfigured_OTel(t *testing.T) {
	tests := []struct {
		label string
		input telemetryv1alpha1.LogPipelineInput
	}{
		{
			label: suite.LabelLogAgent,
			input: testutils.BuildLogPipelineApplicationInput(),
		},
		{
			label: suite.LabelLogGateway,
			input: testutils.BuildLogPipelineOTLPInput(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			var (
				uniquePrefix = unique.Prefix()
				pipelineName = uniquePrefix()
			)

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("name", "namespace", "", "")).
				Build()

			Consistently(func(g Gomega) {
				g.Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, &pipeline)).ShouldNot(Succeed())
			}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	}
}

func TestSecretrefMisconfigured_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
	)

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(testutils.HTTPBasicAuthFromSecret("name", "namespace", "", "")).
		Build()

	Consistently(func(g Gomega) {
		g.Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, &pipeline)).ShouldNot(Succeed())
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
}
