//go:build e2e

package e2e

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

var (
	ctx               context.Context
	cancel            context.CancelFunc
	k8sClient         client.Client
	testEnv           *envtest.Environment
	httpsAuthProvider httpsAuth
	k8sObjects        = []client.Object{k8s.NewTelemetry("default").K8sObject()}
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
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

	scheme := scheme.Scheme
	Expect(telemetryv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(operatorv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
	k8sClient, err = client.New(testEnv.Config, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).To(Succeed())

	// Fetch the authentication-related resources.
	httpsAuthProvider, err = newHTTPSAuth(fetchAuthToken(), apiPort)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
	Eventually(func(g Gomega) {
		var validatingWebhookConfiguration admissionv1.ValidatingWebhookConfiguration
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(BeNil())
		var secret corev1.Secret
		g.Expect(k8sClient.Get(ctx, webhookCertSecret, &secret)).Should(BeNil())
	}, timeout, interval).ShouldNot(Succeed())

	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func fetchAuthToken() string {
	// Fetch the Auth token
	var sa corev1.ServiceAccount
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "testsuite", Namespace: defaultNamespaceName}, &sa)).To(Succeed())
	Expect(sa.Secrets).To(HaveLen(1))

	var secret corev1.Secret
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: sa.Secrets[0].Name, Namespace: defaultNamespaceName}, &secret)).To(Succeed())
	Expect(secret.Data["token"]).NotTo(BeEmpty())

	return string(secret.Data["token"])
}
