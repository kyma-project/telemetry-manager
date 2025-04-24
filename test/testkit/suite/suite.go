package suite

import (
	"context"
	"fmt"
	"path"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
)

const (
	GomegaMaxDepth  = 20
	GomegaMaxLenght = 16_000
)

var (
	Ctx         context.Context
	K8sClient   client.Client
	ProxyClient *apiserverproxy.Client

	cancel            context.CancelFunc
	defaultK8sObjects []client.Object
)

// BeforeSuiteFuncErr is designed to return an error instead of relying on Gomega matchers.
// This function is intended for use in a vanilla TestMain function within new e2e test suites.
// Note that Gomega matchers cannot be utilized in the TestMain function.
func BeforeSuiteFuncErr() error {
	Ctx, cancel = context.WithCancel(context.Background()) //nolint:fatcontext // context is used in tests

	kubeconfigPath := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	K8sClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	ProxyClient, err = apiserverproxy.NewClient(restConfig)

	denyAllNetworkPolicyK8sObject := kitk8s.NewNetworkPolicy("deny-all-ingress-and-egress", kitkyma.SystemNamespaceName).K8sObject()
	defaultK8sObjects = []client.Object{
		denyAllNetworkPolicyK8sObject,
	}

	if err := kitk8s.CreateObjects(Ctx, K8sClient, defaultK8sObjects...); err != nil {
		return fmt.Errorf("failed to create default k8s objects: %w", err)
	}

	return nil
}

// BeforeSuiteFunc is executed before each Ginkgo test suite
func BeforeSuiteFunc() {
	Expect(BeforeSuiteFuncErr()).Should(Succeed())
}

// AfterSuiteFuncErr is designed to return an error instead of relying on Gomega matchers.
// This function is intended for use in a vanilla TestMain function within new e2e test suites.
// Note that Gomega matchers cannot be utilized in the TestMain function.
func AfterSuiteFuncErr() error {
	if err := kitk8s.DeleteObjects(Ctx, K8sClient, defaultK8sObjects...); err != nil {
		return fmt.Errorf("failed to delete default k8s objects: %w", err)
	}

	cancel()

	return nil
}

// AfterSuiteFunc is executed after each Ginkgo test suite
func AfterSuiteFunc() {
	Expect(AfterSuiteFuncErr()).Should(Succeed())
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
	folderName := path.Base(path.Dir(filePath))
	specID := folderName + "-" + strings.TrimSuffix(fileName, "_test.go")
	specID = strings.ReplaceAll(specID, "_", "-")

	return specID
}

const (
	// Test suites labels
	LabelLogsOtel      = "logs-otel"
	LabelLogsFluentBit = "logs-fluentbit"
	LabelTraces        = "traces"
	LabelMetrics       = "metrics"
	LabelTelemetry     = "telemetry"
	LabelMaxPipeline   = "max-pipeline"

	// Test "sub-suites" labels
	LabelExperimental = "experimental"
	LabelSetA         = "set_a"
	LabelSetB         = "set_b"
	LabelSetC         = "set_c"
	LabelSignalPush   = "signal-push"
	LabelSignalPull   = "signal-pull"

	// Self-monitoring test labels
	LabelSelfMonitoringLogsHealthy         = "self-mon-logs-healthy"
	LabelSelfMonitoringLogsBackpressure    = "self-mon-logs-backpressure"
	LabelSelfMonitoringLogsOutage          = "self-mon-logs-outage"
	LabelSelfMonitoringTracesHealthy       = "self-mon-traces-healthy"
	LabelSelfMonitoringTracesBackpressure  = "self-mon-traces-backpressure"
	LabelSelfMonitoringTracesOutage        = "self-mon-traces-outage"
	LabelSelfMonitoringMetricsHealthy      = "self-mon-metrics-healthy"
	LabelSelfMonitoringMetricsBackpressure = "self-mon-metrics-backpressure"
	LabelSelfMonitoringMetricsOutage       = "self-mon-metrics-outage"

	// Miscellaneous test label (for edge-cases and unrelated tests)
	// [please avoid adding tests to this category if it already fits in a more specific one]
	LabelMisc = "misc"

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
