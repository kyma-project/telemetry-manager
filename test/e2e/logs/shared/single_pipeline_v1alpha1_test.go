package shared

import (
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestSinglePipelineV1Alpha1_OTel(t *testing.T) {
	tests := []struct {
		label               string
		input               telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
		expectAgent         bool
	}{
		{
			label: suite.LabelLogAgent,
			input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					Enabled: ptr.To(true),
				},
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return stdoutloggen.NewDeployment(ns).K8sObject()
			},
			expectAgent: true,
		},
		{
			label: suite.LabelLogGateway,
			input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					Enabled: ptr.To(false),
				},
				OTLP: &telemetryv1alpha1.OTLPInput{
					Disabled: false,
				},
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix("pipeline")
				genNs        = uniquePrefix("gen")
				backendNs    = uniquePrefix("backend")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

			// creating a log pipeline explicitly since the testutils.LogPipelineBuilder is not available in the v1beta1 API
			pipeline := telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: pipelineName,
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: tc.input,
					Output: telemetryv1alpha1.LogPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Endpoint: telemetryv1alpha1.ValueType{
								Value: backend.Host() + ":" + strconv.Itoa(int(backend.Port())),
							},
							Protocol: telemetryv1alpha1.OTLPProtocolGRPC,
							TLS: &telemetryv1alpha1.OTLPTLS{
								Insecure:           true,
								InsecureSkipVerify: true,
							},
						},
					},
				},
			}

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
				tc.logGeneratorBuilder(genNs),
			}
			resources = append(resources, backend.K8sObjects()...)

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

func TestSinglePipelineV1Alpha1_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix("logs")
		pipelineName = uniquePrefix()
		genNs        = uniquePrefix("gen")
		backendNs    = uniquePrefix("backend")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)

	pipeline := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineName,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				FluentBitHTTP: &telemetryv1alpha1.FluentBitHTTPOutput{
					Host: telemetryv1alpha1.ValueType{
						Value: backend.Host(),
					},
					Port: strconv.Itoa(int(backend.Port())),
					URI:  "/",
					TLS: telemetryv1alpha1.FluentBitHTTPOutputTLS{
						Disabled:                  true,
						SkipCertificateValidation: true,
					},
				},
			},
		},
	}

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		stdoutloggen.NewDeployment(genNs).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.FluentBitLogPipelineHealthy(t, pipelineName)
	assert.LogPipelineUnsupportedMode(t, pipelineName, false)
}
