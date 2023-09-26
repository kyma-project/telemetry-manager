//go:build e2e

package e2e

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/logproducer"
)

var _ = Describe("Logs", Label("logging"), func() {
	const (
		mockNs          = "log-parser-mocks"
		mockBackendName = "log-receiver-parser"
		logProducerName = "log-producer"
		configParser    = `Format regex
Regex  ^(?<user>[^ ]*) (?<pass>[^ ]*)$
Time_Key time
Time_Format %d/%b/%Y:%H:%M:%S %z
Types user:string pass:string`
	)
	var telemetryExportURL string

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeLogs)
		mockLogProducer := logproducer.New(logProducerName, mockNs).
			WithAnnotations(map[string]string{"fluentbit.io/parser": "my-regex-parser"})
		objs = append(objs, mockBackend.K8sObjects()...)
		objs = append(objs, mockLogProducer.K8sObject(kitk8s.WithLabel("app", "regex-parser-testing-service")))
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		logHTTPPipeline := kitlog.NewPipeline("pipeline-regex-parser").
			WithSecretKeyRef(mockBackend.HostSecretRef()).
			WithHTTPOutput()
		logRegExParser := kitlog.NewParser("my-regex-parser", configParser)
		objs = append(objs, logHTTPPipeline.K8sObject())
		objs = append(objs, logRegExParser.K8sObject())

		return objs
	}

	Context("When a LogParser exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: mockBackendName})
		})

		It("Should have a log producer running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: logProducerName})
		})

		It("Should parse the logs using regex", func() {
			Eventually(func(g Gomega) {
				time.Sleep(20 * time.Second)
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainLogs(WithAttributeKeyValue("user", "foo")),
					ContainLogs(WithAttributeKeyValue("pass", "bar")),
				)))
			}, timeout, interval).Should(Succeed())
		})
	})
})
