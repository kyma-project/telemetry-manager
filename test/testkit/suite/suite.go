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
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
)

var (
	Ctx         context.Context
	K8sClient   client.Client
	ProxyClient *apiserverproxy.Client
)

// BeforeSuiteFunc is designed to return an error instead of relying on Gomega matchers.
// This function is intended for use in a vanilla TestMain function within new e2e test suites.
// Note that Gomega matchers cannot be utilized in the TestMain function.
func BeforeSuiteFunc() error {
	Ctx = context.Background() //nolint:fatcontext // context is used in tests

	// TODO: set up stdout and stderr loggers
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
	// Logs labels

	LabelLogsMisc             = "logs-misc"
	LabelLogAgent             = "log-agent"
	LabelLogGateway           = "log-gateway"
	LabelFluentBit            = "fluent-bit"
	LabelOTelMaxPipeline      = "otel-max-pipeline"
	LabelFluentBitMaxPipeline = "fluent-bit-max-pipeline"
	LabelLogsMaxPipeline      = "logs-max-pipeline"

	// Metrics labels

	LabelMetricsMisc        = "metrics-misc"
	LabelMetricsMaxPipeline = "metrics-max-pipeline"
	LabelMetricAgentSetA    = "metric-agent-a"
	LabelMetricAgentSetB    = "metric-agent-b"
	LabelMetricAgentSetC    = "metric-agent-c"
	LabelMetricGatewaySetA  = "metric-gateway-a"
	LabelMetricGatewaySetB  = "metric-gateway-b"
	LabelMetricGatewaySetC  = "metric-gateway-c"

	// Traces labels

	LabelTraces            = "traces"
	LabelTracesMaxPipeline = "traces-max-pipeline"

	// Telemetry labels

	LabelTelemetry = "telemetry"

	// Test "sub-suites" labels

	LabelExperimental = "experimental"
	LabelSkip         = "skip"

	// Selfmonitor test labels

	// Prefixes for self-monitor test labels

	LabelSelfMonitorLogAgentPrefix      = "selfmonitor-log-agent"
	LabelSelfMonitorLogGatewayPrefix    = "selfmonitor-log-gateway"
	LabelSelfMonitorFluentBitPrefix     = "selfmonitor-fluent-bit"
	LabelSelfMonitorMetricAgentPrefix   = "selfmonitor-metric-agent"
	LabelSelfMonitorMetricGatewayPrefix = "selfmonitor-metric-gateway"
	LabelSelfMonitorTracesPrefix        = "selfmonitor-traces"

	// Prefix custom label/annotation tests

	LabelCustomLabelAnnotation = "custom-label-annotation"

	// Suffixes (representing different scenarios) for self-monitor test labels

	LabelSelfMonitorHealthySuffix      = "healthy"
	LabelSelfMonitorBackpressureSuffix = "backpressure"
	LabelSelfMonitorOutageSuffix       = "outage"

	// LabelMisc defines the label for miscellaneous tests (for edge-cases and unrelated tests)
	// [please avoid adding tests to this category if it already fits in a more specific one]
	LabelMisc = "misc"

	// LabelIstio defines the label for Istio Integration tests
	LabelIstio = "istio"

	// LabelGardener defines the label for Gardener Integration tests
	LabelGardener = "gardener"

	// LabelUpgrade defines the label for Upgrade tests, which preserve K8s objects between test runs.
	LabelUpgrade = "upgrade"

	// LabelOAuth2 defines the label for OAuth2 related tests.
	LabelOAuth2 = "oauth2"
	// LabelMTLS defines the label for mTLS related tests.
	LabelMTLS = "mtls"
)

func ExpectAgent(label string) bool { // TODO(TeodorSAP): Use this for log e2e tests as well
	return label == LabelMetricAgentSetA ||
		label == LabelMetricAgentSetB ||
		label == LabelMetricAgentSetC ||
		label == LabelLogAgent
}

func DebugObjectsEnabled() bool {
	debugEnv := os.Getenv("DEBUG_TEST_OBJECTS")
	return debugEnv == "1" || strings.ToLower(debugEnv) == "true"
}

func RegisterTestCase(t *testing.T, labels ...string) {
	RegisterTestingT(t)

	labelSet := toSet(labels)

	// Skip test if it contains "skipped" label
	if _, exists := labelSet[LabelSkip]; exists {
		t.Skip()
	}

	labelFilterExpr := findLabelFilterExpression()
	doNotExecute := findDoNotExecuteFlag()

	// If no filter is specified, run all tests (unless do-not-execute is set)
	if labelFilterExpr == "" {
		if doNotExecute {
			printTestInfo(t, labels, "would execute (no filter)")
			t.Skip()
		}

		return
	}

	shouldRun, err := evaluateLabelExpression(labels, labelFilterExpr)
	if err != nil {
		t.Fatalf("Invalid label filter: %v", err)
	}

	if doNotExecute {
		if shouldRun {
			printTestInfo(t, labels, fmt.Sprintf("would execute (matches filter: %s)", labelFilterExpr))
		} else {
			printTestInfo(t, labels, fmt.Sprintf("would skip (doesn't match filter: %s)", labelFilterExpr))
		}

		t.Skip()

		return
	}

	if !shouldRun {
		t.Skipf("Test skipped: label filter '%s' not satisfied", labelFilterExpr)
	}
}

func findDoNotExecuteFlag() bool {
	for _, arg := range os.Args {
		if arg == "-do-not-execute" || arg == "--do-not-execute" {
			return true
		}
	}

	return false
}

func printTestInfo(t *testing.T, labels []string, action string) {
	testName := t.Name()

	if testName == "" {
		// Try to get test name from runtime if not available
		if pc, _, _, ok := runtime.Caller(2); ok {
			if fn := runtime.FuncForPC(pc); fn != nil {
				testName = fn.Name()
				// Extract just the test function name
				if parts := strings.Split(testName, "."); len(parts) > 0 {
					testName = parts[len(parts)-1]
				}
			}
		}

		if testName == "" {
			testName = "<unknown test>"
		}
	}

	fmt.Printf("[DRY-RUN] Test: %s | Labels: %v | Action: %s\n", testName, labels, action) //nolint:forbidigo // using fmt for test info output
}

func findLabelFilterExpression() string {
	const prefix = "-labels="

	var labelsArg string

	for _, arg := range os.Args {
		if strings.HasPrefix(arg, prefix) {
			labelsArg = arg
		}
	}

	if labelsArg == "" {
		return ""
	}

	labelsKV := strings.SplitN(labelsArg, "=", 2)
	if len(labelsKV) != 2 {
		return ""
	}

	return labelsKV[1]
}

func toSet(labels []string) map[string]struct{} {
	set := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		if label == "" {
			continue
		}

		set[label] = struct{}{}
	}

	return set
}
