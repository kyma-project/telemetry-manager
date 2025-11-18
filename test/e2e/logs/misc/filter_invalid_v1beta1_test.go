package misc

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestFilterV1Beta1Invalid(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogsMisc)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
	)

	pipeline := telemetryv1beta1.LogPipeline{
		ObjectMeta: v1.ObjectMeta{
			Name: pipelineName,
		},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Filters: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{
						`Len(resource.attributes["k8s.namespace.name"]) > 0`,
						`attributes["foo"] == "bar"`,
					},
				},
			},
			Output: telemetryv1beta1.LogPipelineOutput{
				OTLP: &telemetryv1beta1.OTLPOutput{
					Endpoint: telemetryv1beta1.ValueType{
						Value: "https://backend.example.com:4317",
					},
				},
			},
		},
	}

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(&pipeline)).Should(MatchError(ContainSubstring("not found")))
	})
	Expect(kitk8s.CreateObjects(t, &pipeline)).ToNot(Succeed())
}
