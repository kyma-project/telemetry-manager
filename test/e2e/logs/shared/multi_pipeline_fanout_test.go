package shared

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineFanout_OTel(t *testing.T) {
	tests := []struct {
		name                string
		labels              []string
		inputBuilder        func(includeNs string) telemetryv1beta1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
	}{
		{
			name:   suite.LabelLogAgent,
			labels: []string{suite.LabelLogAgent},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return stdoutloggen.NewDeployment(ns).K8sObject()
			},
		},
		{
			name:   suite.LabelLogGateway,
			labels: []string{suite.LabelLogGateway},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},
		{
			name:   fmt.Sprintf("%s-%s", suite.LabelLogGateway, suite.LabelExperimental),
			labels: []string{suite.LabelLogGateway, suite.LabelExperimental},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeCentralLogs).K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.labels...)

			var (
				uniquePrefix  = unique.Prefix(tc.name)
				backendNs     = uniquePrefix("backend")
				genNs         = uniquePrefix("gen")
				pipeline1Name = uniquePrefix("pipeline1")
				pipeline2Name = uniquePrefix("pipeline2")
			)

			backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend1"))
			backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend2"))

			pipeline1 := testutils.NewLogPipelineBuilder().
				WithName(pipeline1Name).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend1.EndpointHTTP())).
				Build()

			pipeline2 := testutils.NewLogPipelineBuilder().
				WithName(pipeline2Name).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend2.EndpointHTTP())).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline1,
				&pipeline2,
				tc.logGeneratorBuilder(genNs),
			}
			resources = append(resources, backend1.K8sObjects()...)
			resources = append(resources, backend2.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend1)
			assert.BackendReachable(t, backend2)
			assert.OTelLogPipelineHealthy(t, pipeline1.Name)
			assert.OTelLogPipelineHealthy(t, pipeline2.Name)
			assert.OTelLogsFromNamespaceDelivered(t, backend1, genNs)
			assert.OTelLogsFromNamespaceDelivered(t, backend2, genNs)
		})
	}
}

func TestMultiPipelineFanout_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix  = unique.Prefix()
		backendNs     = uniquePrefix("backend")
		genNs         = uniquePrefix("gen")
		pipeline1Name = uniquePrefix("pipeline1")
		pipeline2Name = uniquePrefix("pipeline2")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend1"))
	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend2"))

	pipeline1 := testutils.NewLogPipelineBuilder().
		WithName(pipeline1Name).
		WithRuntimeInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend1.Host()), testutils.HTTPPort(backend1.Port())).
		Build()

	pipeline2 := testutils.NewLogPipelineBuilder().
		WithName(pipeline2Name).
		WithRuntimeInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend2.Host()), testutils.HTTPPort(backend2.Port())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		&pipeline1,
		&pipeline2,
		stdoutloggen.NewDeployment(genNs).K8sObject(),
	}
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend1)
	assert.BackendReachable(t, backend2)
	assert.FluentBitLogPipelineHealthy(t, pipeline1.Name)
	assert.FluentBitLogPipelineHealthy(t, pipeline2.Name)
	assert.FluentBitLogsFromNamespaceDelivered(t, backend1, genNs)
	assert.FluentBitLogsFromNamespaceDelivered(t, backend2, genNs)
}
