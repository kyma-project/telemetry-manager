package shared

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
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

func TestNamespaceSelector_OTel(t *testing.T) {
	tests := []struct {
		name                string
		labels              []string
		inputBuilder        func(includeNss, excludeNss []string) telemetryv1beta1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
		resourceName        types.NamespacedName
		readinessCheckFunc  func(t *testing.T, name types.NamespacedName)
	}{
		{
			name:   suite.LabelLogAgent,
			labels: []string{suite.LabelLogAgent},
			inputBuilder: func(includeNss, excludeNss []string) telemetryv1beta1.LogPipelineInput {
				var opts []testutils.NamespaceSelectorOptions
				if len(includeNss) > 0 {
					opts = append(opts, testutils.IncludeNamespaces(includeNss...))
				}

				if len(excludeNss) > 0 {
					opts = append(opts, testutils.ExcludeNamespaces(excludeNss...))
				}

				return testutils.BuildLogPipelineRuntimeInput(opts...)
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return stdoutloggen.NewDeployment(ns).K8sObject()
			},
			resourceName:       kitkyma.LogAgentName,
			readinessCheckFunc: assert.DaemonSetReady,
		},
		{
			name:   suite.LabelLogGateway,
			labels: []string{suite.LabelLogGateway},
			inputBuilder: func(includeNss, excludeNss []string) telemetryv1beta1.LogPipelineInput {
				var opts []testutils.NamespaceSelectorOptions
				if len(includeNss) > 0 {
					opts = append(opts, testutils.IncludeNamespaces(includeNss...))
				}

				if len(excludeNss) > 0 {
					opts = append(opts, testutils.ExcludeNamespaces(excludeNss...))
				}

				return testutils.BuildLogPipelineOTLPInput(opts...)
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
			resourceName:       kitkyma.LogGatewayName,
			readinessCheckFunc: assert.DeploymentReady,
		},
		{
			name:   fmt.Sprintf("%s-%s", suite.LabelLogGateway, suite.LabelExperimental),
			labels: []string{suite.LabelLogGateway, suite.LabelExperimental},
			inputBuilder: func(includeNss, excludeNss []string) telemetryv1beta1.LogPipelineInput {
				var opts []testutils.NamespaceSelectorOptions
				if len(includeNss) > 0 {
					opts = append(opts, testutils.IncludeNamespaces(includeNss...))
				}

				if len(excludeNss) > 0 {
					opts = append(opts, testutils.ExcludeNamespaces(excludeNss...))
				}

				return testutils.BuildLogPipelineOTLPInput(opts...)
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeCentralLogs).K8sObject()
			},
			resourceName:       kitkyma.TelemetryOTLPGatewayName,
			readinessCheckFunc: assert.DaemonSetReady,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.labels...)

			var (
				uniquePrefix        = unique.Prefix(tc.name)
				gen1Ns              = uniquePrefix("gen-1")
				includePipelineName = uniquePrefix("include")
				gen2Ns              = uniquePrefix("gen-2")
				excludePipelineName = uniquePrefix("exclude")
				backendNs           = uniquePrefix("backend")
			)

			backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend-1"))
			backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend-2"))

			// Include gen1Ns only
			includePipeline := testutils.NewLogPipelineBuilder().
				WithName(includePipelineName).
				WithInput(tc.inputBuilder([]string{gen1Ns}, nil)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend1.EndpointHTTP())).
				Build()

			// Exclude all namespaces except gen1Ns (gen2Ns and other unrelated namespaces)
			// to avoid implicitly collecting logs from other namespaces
			// and potentially overloading the backend.
			var nsList corev1.NamespaceList

			Expect(suite.K8sClient.List(t.Context(), &nsList)).To(Succeed())

			excludeNss := []string{gen2Ns}

			for _, namespace := range nsList.Items {
				if namespace.Name != gen1Ns && namespace.Name != gen2Ns {
					excludeNss = append(excludeNss, namespace.Name)
				}
			}

			excludePipeline := testutils.NewLogPipelineBuilder().
				WithName(excludePipelineName).
				WithInput(tc.inputBuilder(nil, excludeNss)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend2.EndpointHTTP())).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(gen1Ns).K8sObject(),
				kitk8sobjects.NewNamespace(gen2Ns).K8sObject(),
				&includePipeline,
				&excludePipeline,
				tc.logGeneratorBuilder(gen1Ns),
				tc.logGeneratorBuilder(gen2Ns),
			}
			resources = append(resources, backend1.K8sObjects()...)
			resources = append(resources, backend2.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend1)
			assert.BackendReachable(t, backend2)

			tc.readinessCheckFunc(t, tc.resourceName)

			assert.OTelLogPipelineHealthy(t, includePipelineName)
			assert.OTelLogPipelineHealthy(t, excludePipelineName)

			assert.OTelLogsFromNamespaceDelivered(t, backend1, gen1Ns)
			assert.OTelLogsFromNamespaceNotDelivered(t, backend2, gen2Ns)
		})
	}
}

func TestNamespaceSelector_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix        = unique.Prefix()
		gen1Ns              = uniquePrefix("gen-1")
		includePipelineName = uniquePrefix("include")
		gen2Ns              = uniquePrefix("gen-2")
		excludePipelineName = uniquePrefix("exclude")
		backendNs           = uniquePrefix("backend")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend-1"))
	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend-2"))

	includePipeline := testutils.NewLogPipelineBuilder().
		WithName(includePipelineName).
		WithRuntimeInput(true, testutils.IncludeNamespaces(gen1Ns)).
		WithHTTPOutput(testutils.HTTPHost(backend1.Host()), testutils.HTTPPort(backend1.Port())).
		Build()

	excludeGen2Pipeline := testutils.NewLogPipelineBuilder().
		WithName(excludePipelineName).
		WithRuntimeInput(true, testutils.ExcludeNamespaces(gen2Ns)).
		WithHTTPOutput(testutils.HTTPHost(backend2.Host()), testutils.HTTPPort(backend2.Port())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(gen1Ns).K8sObject(),
		kitk8sobjects.NewNamespace(gen2Ns).K8sObject(),
		&includePipeline,
		&excludeGen2Pipeline,
		stdoutloggen.NewDeployment(gen1Ns).K8sObject(),
		stdoutloggen.NewDeployment(gen2Ns).K8sObject(),
	}
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend1)
	assert.BackendReachable(t, backend2)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.FluentBitLogPipelineHealthy(t, includePipelineName)
	assert.FluentBitLogPipelineHealthy(t, excludePipelineName)
	assert.FluentBitLogsFromNamespaceDelivered(t, backend1, gen1Ns)
	assert.FluentBitLogsFromNamespaceNotDelivered(t, backend2, gen2Ns)
}
