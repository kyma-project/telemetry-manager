//go:build istio

package istio

import (
	"context"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/apiserver"
	istiosecv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"

	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

const (
	// A label that designates a test as an operational one.
	// Operational tests preserve K8s objects between test runs.
	operationalTest = "operational"
)

var (
	webhookName       = "validation.webhook.telemetry.kyma-project.io"
	webhookCertSecret = types.NamespacedName{
		Name:      "telemetry-webhook-cert",
		Namespace: kitkyma.SystemNamespaceName,
	}
)

var (
	ctx                 context.Context
	cancel              context.CancelFunc
	k8sClient           client.Client
	proxyClient         *apiserver.ProxyClient
	testEnv             *envtest.Environment
	telemetryK8sObjects []client.Object
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

	scheme := scheme.Scheme
	Expect(telemetryv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(operatorv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(istiosecv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
	k8sClient, err = client.New(testEnv.Config, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	telemetryK8sObjects = []client.Object{kitk8s.NewTelemetry("default", "kyma-system").Persistent(isOperational()).K8sObject()}

	Expect(kitk8s.CreateObjects(ctx, k8sClient, telemetryK8sObjects...)).To(Succeed())

	proxyClient, err = apiserver.NewProxyClient(testEnv.Config)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(kitk8s.DeleteObjects(ctx, k8sClient, telemetryK8sObjects...)).Should(Succeed())
	if !isOperational() {
		Eventually(func(g Gomega) {
			var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(Succeed())
			var secret corev1.Secret
			g.Expect(k8sClient.Get(ctx, webhookCertSecret, &secret)).Should(Succeed())
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(Succeed())
	}

	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// isOperational returns true if the test is invoked with an "operational" tag.
func isOperational() bool {
	labelsFilter := GinkgoLabelFilter()

	return labelsFilter != "" && Label(operationalTest).MatchesLabelFilter(labelsFilter)
}
