//go:build e2e

package otel

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

type testCase struct {
	namePrefix      string
	logPipelineFunc func(builder *testutils.LogPipelineBuilder, includeNs []string, excludeNs []string) *testutils.LogPipelineBuilder
	logProducerFunc func(ns string) client.Object
}

func (tc *testCase) name(name string) string {
	return suite.IDWithSuffix(fmt.Sprintf("%s-%s", tc.namePrefix, name))
}

var _ = DescribeTable(suite.ID(), Label(suite.LabelLogsOtel, suite.LabelSignalPull, suite.LabelExperimental), Ordered, func(testCase testCase) {
	var (
		mockNs                  = testCase.name("mocks")
		app1Ns                  = testCase.name("app-1")
		app2Ns                  = testCase.name("app-2")
		backend1Name            = testCase.name("backend-include")
		backendIncludeExportURL string
		backend2Name            = testCase.name("backend-exclude")
		backendExcludeExportURL string
	)

	makeResources := func() []client.Object {
		backendInclude := backend.New(mockNs, backend.SignalTypeLogsOtel, backend.WithName(backend1Name))
		backendIncludeExportURL = backendInclude.ExportURL(suite.ProxyClient)

		backendExclude := backend.New(mockNs, backend.SignalTypeLogsOtel, backend.WithName(backend2Name))
		backendExcludeExportURL = backendExclude.ExportURL(suite.ProxyClient)

		pipelineInclude := testCase.logPipelineFunc(
			testutils.NewLogPipelineBuilder().
				WithName(testCase.name("include")).
				WithOTLPOutput(testutils.OTLPEndpoint(backendInclude.Endpoint())),
			[]string{app1Ns},
			[]string{},
		).Build()

		pipelineExclude := testCase.logPipelineFunc(
			testutils.NewLogPipelineBuilder().
				WithName(testCase.name("exclude")).
				WithOTLPOutput(testutils.OTLPEndpoint(backendExclude.Endpoint())),
			[]string{},
			[]string{app1Ns},
		).Build()

		var objs []client.Object
		objs = append(objs, backendInclude.K8sObjects()...)
		objs = append(objs, backendExclude.K8sObjects()...)
		objs = append(objs,
			kitk8s.NewNamespace(mockNs).K8sObject(),
			kitk8s.NewNamespace(app1Ns).K8sObject(),
			kitk8s.NewNamespace(app2Ns).K8sObject(),
			&pipelineInclude,
			&pipelineExclude,
			testCase.logProducerFunc(app1Ns),
			testCase.logProducerFunc(app2Ns),
		)

		return objs
	}

	k8sObjects := makeResources()
	DeferCleanup(func() {
		Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
	})

	Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())

	assert.DeploymentReady(suite.Ctx, suite.K8sClient, kitkyma.LogGatewayName)
	assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend1Name, Namespace: mockNs})
	assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend2Name, Namespace: mockNs})
	assert.ServiceReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend1Name, Namespace: mockNs})
	assert.ServiceReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend2Name, Namespace: mockNs})
	assert.LogsFromNamespaceDelivered(suite.ProxyClient, backendIncludeExportURL, app1Ns)
	assert.LogsFromNamespaceNotDelivered(suite.ProxyClient, backendIncludeExportURL, app2Ns)
	assert.LogsFromNamespaceDelivered(suite.ProxyClient, backendExcludeExportURL, app2Ns)
	assert.LogsFromNamespaceNotDelivered(suite.ProxyClient, backendExcludeExportURL, app1Ns)
},
	Entry("otlp", testCase{
		namePrefix: "otlp",
		logPipelineFunc: func(builder *testutils.LogPipelineBuilder, includeNs []string, excludeNs []string) *testutils.LogPipelineBuilder {
			return builder.
				WithApplicationInput(false).
				WithOTLPInput(true, testutils.IncludeNamespaces(includeNs...), testutils.ExcludeNamespaces(excludeNs...))
		},
		logProducerFunc: func(ns string) client.Object {
			podSpec := telemetrygen.PodSpec(telemetrygen.SignalTypeLogs)
			return kitk8s.NewDeployment(ns, ns).WithPodSpec(podSpec).K8sObject()
		},
	}),
	Entry("application", testCase{
		namePrefix: "application",
		logPipelineFunc: func(builder *testutils.LogPipelineBuilder, includeNs []string, excludeNs []string) *testutils.LogPipelineBuilder {
			return builder.
				WithOTLPInput(false).
				WithApplicationInput(true, testutils.IncludeLogNamespaces(includeNs...), testutils.ExcludeLogNamespaces(excludeNs...))
		},
		logProducerFunc: func(ns string) client.Object {
			return loggen.New(ns).K8sObject()
		},
	}),
)
