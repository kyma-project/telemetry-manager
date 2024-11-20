//go:build istio

package istio

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var (
	ctx                context.Context
	cancel             context.CancelFunc
	k8sClient          client.Client
	proxyClient        *apiserverproxy.Client
	testEnv            *envtest.Environment
	telemetryK8sObject client.Object
	k8sObjects         []client.Object
)

func TestIstioIntegration(t *testing.T) {
	format.MaxDepth = 20
	format.MaxLength = 16000
	RegisterFailHandler(Fail)
	RunSpecs(t, "Istio Integration Suite")
}

var _ = BeforeSuite(func() {
	var err error
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	useExistingCluster := true
	testEnv = &envtest.Environment{
		UseExistingCluster: &useExistingCluster,
	}

	_, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")

	scheme := clientgoscheme.Scheme
	Expect(telemetryv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(operatorv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(istiosecurityclientv1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(istionetworkingclientv1.AddToScheme(scheme)).NotTo(HaveOccurred())
	k8sClient, err = client.New(testEnv.Config, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	telemetryK8sObject = kitk8s.NewTelemetry("default", "kyma-system").Persistent(suite.IsOperational()).K8sObject()
	denyAllNetworkPolicyK8sObject := kitk8s.NewNetworkPolicy("deny-all-ingress-and-egress", kitkyma.SystemNamespaceName).K8sObject()
	k8sObjects = []client.Object{
		telemetryK8sObject,
		denyAllNetworkPolicyK8sObject,
	}

	Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).To(Succeed())

	proxyClient, err = apiserverproxy.NewClient(testEnv.Config)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
	if !suite.IsOperational() {
		Eventually(func(g Gomega) {
			var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: kitkyma.WebhookName}, &validatingWebhookConfiguration)).Should(Succeed())
			var secret corev1.Secret
			g.Expect(k8sClient.Get(ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(Succeed())
	}

	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
