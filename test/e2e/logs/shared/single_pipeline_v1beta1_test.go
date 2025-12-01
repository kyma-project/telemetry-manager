package shared

import (
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestSinglePipelineV1Beta1_OTel(t *testing.T) {
	tests := []struct {
		prefix              string
		input               telemetryv1beta1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
		expectAgent         bool
	}{
		{
			prefix: "agent",
			input:  testutils.BuildLogPipelineV1Beta1RuntimeInput(),
			logGeneratorBuilder: func(ns string) client.Object {
				return stdoutloggen.NewDeployment(ns).K8sObject()
			},
			expectAgent: true,
		},
		{
			prefix: "gateway",
			input:  testutils.BuildLogPipelineV1Beta1OTLPInput(),
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			suite.RegisterTestCase(t, suite.LabelExperimental)

			var (
				uniquePrefix = unique.Prefix("logs", tc.prefix)
				pipelineName = uniquePrefix("pipeline")
				genNs        = uniquePrefix("gen")
				backendNs    = uniquePrefix("backend")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

			// creating a log pipeline explicitly since the testutils.LogPipelineBuilder is not available in the v1beta1 API
			pipeline := telemetryv1beta1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: pipelineName,
				},
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: tc.input,
					Output: telemetryv1beta1.LogPipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{
							Endpoint: telemetryv1beta1.ValueType{
								Value: backend.Host() + ":" + strconv.Itoa(int(backend.Port())),
							},
							Protocol: telemetryv1beta1.OTLPProtocolGRPC,
							TLS: &telemetryv1beta1.OutputTLS{
								Insecure:           true,
								InsecureSkipVerify: true,
							},
						},
					},
				},
			}

			resources := []client.Object{
				objects.NewNamespace(backendNs).K8sObject(),
				objects.NewNamespace(genNs).K8sObject(),
				&pipeline,
				tc.logGeneratorBuilder(genNs),
			}
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
			})
			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.OTelLogPipelineHealthy(t, pipelineName)
			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.LogGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
			}

			assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)
		})
	}
}

func TestSinglePipelineV1Beta1_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelExperimental, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix("logs")
		pipelineName = uniquePrefix()
		genNs        = uniquePrefix("gen")
		backendNs    = uniquePrefix("backend")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)

	pipeline := telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineName,
		},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Output: telemetryv1beta1.LogPipelineOutput{
				HTTP: &telemetryv1beta1.LogPipelineHTTPOutput{
					Host: telemetryv1beta1.ValueType{
						Value: backend.Host(),
					},
					Port: strconv.Itoa(int(backend.Port())),
					URI:  "/",
					TLSConfig: telemetryv1beta1.OutputTLS{
						Insecure:           true,
						InsecureSkipVerify: true,
					},
				},
			},
		},
	}

	resources := []client.Object{
		objects.NewNamespace(backendNs).K8sObject(),
		objects.NewNamespace(genNs).K8sObject(),
		stdoutloggen.NewDeployment(genNs).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.FluentBitLogPipelineHealthy(t, pipelineName)
	assert.LogPipelineUnsupportedMode(t, pipelineName, false)
	assert.FluentBitLogsFromNamespaceDelivered(t, backend, genNs)
}
