package suite

import (
	"context"
	"path"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
)

const (
	GomegaMaxDepth  = 20
	GomegaMaxLenght = 16_000
)

var (
	Ctx                context.Context
	Cancel             context.CancelFunc
	K8sClient          client.Client
	ProxyClient        *apiserverproxy.Client
	TestEnv            *envtest.Environment
	TelemetryK8sObject client.Object
	k8sObjects         []client.Object
)

// Function to be executed before each e2e suite
func BeforeSuiteFunc() {
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
	k8sObjects = []client.Object{
		TelemetryK8sObject,
		denyAllNetworkPolicyK8sObject,
	}

	Expect(kitk8s.CreateObjects(Ctx, K8sClient, k8sObjects...)).To(Succeed())

	ProxyClient, err = apiserverproxy.NewClient(TestEnv.Config)
	Expect(err).NotTo(HaveOccurred())
}

// Function to be executed after each e2e suite
func AfterSuiteFunc() {
	Expect(kitk8s.DeleteObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())

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
}

// ID returns the current test suite ID.
// It is based on the file name of the test suite.
// It is useful for generating unique names for resources created in the test suite (telemetry pipelines, mock namespaces, etc.).
func ID() string {
	_, filePath, _, ok := runtime.Caller(1)
	if !ok {
		panic("Cannot get the current file path")
	}

	return sanitizeSpecID(filePath)
}

// IDWithSuffix returns the current test suite ID with the provided suffix.
// If no suffix is provided, it defaults to an empty string.
func IDWithSuffix(suffix string) string {
	_, filePath, _, ok := runtime.Caller(1)
	if !ok {
		panic("Cannot get the current file path")
	}

	return sanitizeSpecID(filePath) + "-" + suffix
}

func sanitizeSpecID(filePath string) string {
	fileName := path.Base(filePath)
	specID := strings.TrimSuffix(fileName, "_test.go")
	specID = strings.ReplaceAll(specID, "_", "-")

	return specID
}

const (
	LabelLogs                 = "logs"
	LabelTraces               = "traces"
	LabelMetrics              = "metrics"
	LabelTelemetry            = "telemetry"
	LabelExperimental         = "experimental"
	LabelTelemetryLogAnalysis = "telemetry-log-analysis"
	LabelMaxPipeline          = "max-pipeline"
	LabelSetA                 = "set_a"
	LabelSetB                 = "set_b"
	LabelSetC                 = "set_c"

	LabelSelfMonitoringLogsHealthy         = "self-mon-logs-healthy"
	LabelSelfMonitoringLogsBackpressure    = "self-mon-logs-backpressure"
	LabelSelfMonitoringLogsOutage          = "self-mon-logs-outage"
	LabelSelfMonitoringTracesHealthy       = "self-mon-traces-healthy"
	LabelSelfMonitoringTracesBackpressure  = "self-mon-traces-backpressure"
	LabelSelfMonitoringTracesOutage        = "self-mon-traces-outage"
	LabelSelfMonitoringMetricsHealthy      = "self-mon-metrics-healthy"
	LabelSelfMonitoringMetricsBackpressure = "self-mon-metrics-backpressure"
	LabelSelfMonitoringMetricsOutage       = "self-mon-metrics-outage"

	// Istio test label
	LabelIntegration = "integration"

	// Upgrade tests preserve K8s objects between test runs.
	LabelUpgrade = "upgrade"
)

// IsUpgrade returns true if the test is invoked with an "upgrade" tag.
func IsUpgrade() bool {
	labelsFilter := GinkgoLabelFilter()

	return labelsFilter != "" && Label(LabelUpgrade).MatchesLabelFilter(labelsFilter)
}
