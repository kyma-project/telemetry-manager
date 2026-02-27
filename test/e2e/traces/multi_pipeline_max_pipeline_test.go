package traces

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineMaxPipeline(t *testing.T) {
	tests := []struct {
		name         string
		labels       []string
		opts         []kubeprep.Option
		experimental bool
	}{
		{
			name:         "max-pipeline-limit",
			labels:       []string{suite.LabelTracesMaxPipeline, suite.LabelMaxPipeline, suite.LabelTraces},
			experimental: false,
		},
		{
			name:         "unlimited-pipelines-experimental",
			labels:       []string{suite.LabelTracesMaxPipeline, suite.LabelMaxPipeline, suite.LabelTraces},
			opts:         []kubeprep.Option{kubeprep.WithExperimental()},
			experimental: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.SetupTestWithOptions(t, tc.labels, tc.opts...)

			const maxNumberOfTracePipelines = resourcelock.MaxPipelineCount

			var (
				uniquePrefix = unique.Prefix("traces")
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")

				pipelineBase           = uniquePrefix()
				additionalPipelineName = fmt.Sprintf("%s-limit-exceeded", pipelineBase)
				pipelines              []client.Object
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)

			for i := range maxNumberOfTracePipelines {
				pipelineName := fmt.Sprintf("%s-%d", pipelineBase, i)
				pipeline := testutils.NewTracePipelineBuilder().
					WithName(pipelineName).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()
				pipelines = append(pipelines, &pipeline)
			}

			additionalPipeline := testutils.NewTracePipelineBuilder().
				WithName(additionalPipelineName).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				telemetrygen.NewPod(genNs, telemetrygen.SignalTypeTraces).K8sObject(),
			}
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())
			Expect(kitk8s.CreateObjects(t, pipelines...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.TraceGatewayName)

			t.Log("Asserting all pipelines are healthy")

			for _, pipeline := range pipelines {
				assert.TracePipelineHealthy(t, pipeline.GetName())
			}

			t.Log("Attempting to create a pipeline that exceeds the maximum allowed number of pipelines")
			Expect(kitk8s.CreateObjects(t, &additionalPipeline)).To(Succeed())

			if tc.experimental {
				t.Log("Experimental mode: unlimited pipelines enabled, additional pipeline should be healthy")
				assert.TracePipelineHealthy(t, additionalPipelineName)

				t.Log("Verifying traces are delivered for all pipelines")
				assert.TracesFromNamespaceDelivered(t, backend, genNs)
			} else {
				t.Log("Normal mode: verifying max pipeline limit is enforced")
				assert.TracePipelineHasCondition(t, additionalPipelineName, metav1.Condition{
					Type:   conditions.TypeConfigurationGenerated,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonMaxPipelinesExceeded,
				})
				assert.TracePipelineHasCondition(t, additionalPipelineName, metav1.Condition{
					Type:   conditions.TypeFlowHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonConfigNotGenerated,
				})

				t.Log("Verifying traces are delivered for valid pipelines")
				assert.TracesFromNamespaceDelivered(t, backend, genNs)

				t.Log("Deleting one pipeline to free up a slot for the additional pipeline")

				deletePipeline := pipelines[0]
				Expect(kitk8s.DeleteObjects(deletePipeline)).To(Succeed())
				assert.TracePipelineHealthy(t, additionalPipeline.GetName())
			}
		})
	}
}
