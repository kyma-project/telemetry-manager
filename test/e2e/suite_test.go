//go:build e2e

package e2e

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestE2E(t *testing.T) {
	format.MaxDepth = 20
	format.MaxLength = 16_000

	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = BeforeSuite(func() {
	var err error
	logf.SetLogger(logzap.New(logzap.WriteTo(GinkgoWriter), logzap.UseDevMode(true)))
	useExistingCluster := true
	TestEnv = &envtest.Environment{
		UseExistingCluster: &useExistingCluster,
	}

	_, err = TestEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Ctx, Cancel = context.WithCancel(context.Background()) //nolint:fatcontext // context is used in tests

	By("bootstrapping test environment")

	scheme := clientgoscheme.Scheme
	Expect(telemetryv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(telemetryv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(operatorv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
	K8sClient, err = client.New(TestEnv.Config, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(K8sClient).NotTo(BeNil())

	TelemetryK8sObject = kitk8s.NewTelemetry("default", "kyma-system").Persistent(IsUpgrade()).K8sObject()
	denyAllNetworkPolicyK8sObject := kitk8s.NewNetworkPolicy("deny-all-ingress-and-egress", kitkyma.SystemNamespaceName).K8sObject()
	K8sObjects = []client.Object{
		TelemetryK8sObject,
		denyAllNetworkPolicyK8sObject,
	}

	Expect(kitk8s.CreateObjects(Ctx, K8sClient, K8sObjects...)).To(Succeed())

	ProxyClient, err = apiserverproxy.NewClient(TestEnv.Config)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(kitk8s.DeleteObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
	if !IsUpgrade() {
		Eventually(func(g Gomega) {
			var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
			g.Expect(K8sClient.Get(Ctx, client.ObjectKey{Name: kitkyma.ValidatingWebhookName}, &validatingWebhookConfiguration)).Should(Succeed())
			var secret corev1.Secret
			g.Expect(K8sClient.Get(Ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(Succeed())
	}

	Cancel()
	By("tearing down the test environment")
	err := TestEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
