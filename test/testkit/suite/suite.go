package suite

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
)

const (
	GomegaMaxDepth  = 20
	GomegaMaxLenght = 16_000
)

var (
	Ctx         context.Context
	K8sClient   client.Client
	ProxyClient *apiserverproxy.Client

	cancel context.CancelFunc
)

// BeforeSuiteFuncErr is designed to return an error instead of relying on Gomega matchers.
// This function is intended for use in a vanilla TestMain function within new e2e test suites.
// Note that Gomega matchers cannot be utilized in the TestMain function.
func BeforeSuiteFuncErr() error {
	Ctx, cancel = context.WithCancel(context.Background()) //nolint:fatcontext // context is used in tests

	//TODO: set up stdout and stderr loggers
	logf.SetLogger(logr.FromContextOrDiscard(Ctx))

	restConfig, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get k8s config: %w", err)
	}

	K8sClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	ProxyClient, err = apiserverproxy.NewClient(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create apiserver proxy client: %w", err)
	}

	return nil
}

// BeforeSuiteFunc is executed before each Ginkgo test suite
func BeforeSuiteFunc() {
	Expect(BeforeSuiteFuncErr()).Should(Succeed())
}

// AfterSuiteFunc is executed after each Ginkgo test suite
func AfterSuiteFunc() {
	cancel()
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
	LabelLogs                 = "logs"
	LabelLogsOtel             = "logs-otel"
	LabelLogsFluentBit        = "logs-fluentbit"
	LabelLogAgent             = "log-agent"
	LabelLogGateway           = "log-gateway"
	LabelFluentBit            = "fluent-bit"
	LabelTraces               = "traces"
	LabelMetrics              = "metrics"
	LabelTelemetry            = "telemetry"
	LabelMaxPipeline          = "max-pipeline"
	LabelMaxPipelineOTel      = "max-pipeline-otel"
	LabelMaxPipelineFluentBit = "max-pipeline-fluent-bit"

	// Test "sub-suites" labels
	LabelExperimental = "experimental"
	LabelSetA         = "set_a"
	LabelSetB         = "set_b"
	LabelSetC         = "set_c"
	LabelSignalPush   = "signal-push"
	LabelSignalPull   = "signal-pull"
	LabelSkip         = "skip"

	// Self-monitoring test labels
	LabelSelfMonitoringLogsFluentBitBackpressure = "self-mon-logs-fluentbit-backpressure"
	LabelSelfMonitoringLogsFluentBitOutage       = "self-mon-logs-fluentbit-outage"
	LabelSelfMonitoringLogsAgentBackpressure     = "self-mon-logs-agent-backpressure"
	LabelSelfMonitoringLogsAgentOutage           = "self-mon-logs-agent-outage"
	LabelSelfMonitoringLogsGatewayBackpressure   = "self-mon-logs-gateway-backpressure"
	LabelSelfMonitoringLogsGatewayOutage         = "self-mon-logs-gateway-outage"
	LabelSelfMonitoringTracesHealthy             = "self-mon-traces-healthy"
	LabelSelfMonitoringTracesBackpressure        = "self-mon-traces-backpressure"
	LabelSelfMonitoringTracesOutage              = "self-mon-traces-outage"
	LabelSelfMonitoringMetricsHealthy            = "self-mon-metrics-healthy"
	LabelSelfMonitoringMetricsBackpressure       = "self-mon-metrics-backpressure"
	LabelSelfMonitoringMetricsOutage             = "self-mon-metrics-outage"

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

func RegisterTestCase(t *testing.T, labels ...string) {
	RegisterTestingT(t)

	labelSet := toSet(labels)

	requiredLabels := findRequiredLabels()
	if len(requiredLabels) == 0 {
		return
	}

	// Skip test if it contains "skipped" label
	if _, exists := labelSet[LabelSkip]; exists {
		t.Skip()
	}

	// Skip test if it doesn't contain at least one required label
	for _, requiredLabel := range requiredLabels {
		if _, exists := labelSet[requiredLabel]; !exists {
			t.Skip()
		}
	}
}

func findRequiredLabels() []string {
	const prefix = "-labels="

	var labelsArg string

	for _, arg := range os.Args {
		if strings.HasPrefix(arg, prefix) {
			labelsArg = arg
		}
	}

	if labelsArg == "" {
		return nil
	}

	labelsKV := strings.SplitN(labelsArg, "=", 2)
	if len(labelsKV) != 2 {
		return nil
	}

	return strings.Split(labelsKV[1], ",")
}

func toSet(labels []string) map[string]struct{} {
	set := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		set[label] = struct{}{}
	}

	return set
}
