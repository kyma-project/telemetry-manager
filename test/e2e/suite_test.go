//go:build e2e

package e2e

import (
	"context"
	"fmt"
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
	testEnv = &envtest.Environment{
		UseExistingCluster: &useExistingCluster,
	}

	_, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	ctx, cancel = context.WithCancel(context.TODO()) //nolint:fatcontext // context is used in tests

	By("bootstrapping test environment")

	scheme := clientgoscheme.Scheme
	Expect(telemetryv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(telemetryv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(operatorv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
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
	fmt.Printf("Starting AfterSuite cleanup\n")

	err := kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)
	fmt.Printf("DeleteObjects result: %v\n", err)
	Expect(err).Should(Succeed())

	if !suite.IsOperational() {
		fmt.Printf("Suite not operational, checking resources\n")
		Eventually(func(g Gomega) {
			var validatingWebhook admissionregistrationv1.ValidatingWebhookConfiguration
			var secret corev1.Secret

			webhookErr := k8sClient.Get(ctx, client.ObjectKey{Name: kitkyma.ValidatingWebhookName}, &validatingWebhook)
			fmt.Printf("Webhook check: %v\n", webhookErr)

			secretErr := k8sClient.Get(ctx, kitkyma.WebhookCertSecret, &secret)
			fmt.Printf("Secret check: %v\n", secretErr)

			g.Expect(webhookErr).Should(Succeed())
			g.Expect(secretErr).Should(Succeed())
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(Succeed())
	}

	fmt.Printf("Cancelling context and stopping test environment\n")
	cancel()
	err = testEnv.Stop()
	fmt.Printf("TestEnv stop result: %v\n", err)
	Expect(err).NotTo(HaveOccurred())
})
