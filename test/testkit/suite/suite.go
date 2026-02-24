package suite

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
)

const (
	// DefaultLocalImage is used when MANAGER_IMAGE is not set.
	// This allows local development without setting environment variables.
	DefaultLocalImage = "telemetry-manager:latest"
)

var (
	Ctx         context.Context
	K8sClient   client.Client
	ProxyClient *apiserverproxy.Client
)

var (
	labelFilterFlag  string
	doNotExecuteFlag bool
	printLabelsFlag  bool
)

// Environment-affecting labels - these determine cluster setup
var environmentLabels = map[string]bool{
	LabelIstio:        true,
	LabelExperimental: true,
	LabelNoFIPS:       true,
}

// BeforeSuiteFunc is designed to return an error instead of relying on Gomega matchers.
// This function is intended for use in a vanilla TestMain function within new e2e test suites.
// Note that Gomega matchers cannot be utilized in the TestMain function.
//
// This function only initializes the K8s client and context. Cluster preparation
// is handled dynamically by SetupTest based on test labels.
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

func AfterSuiteFunc() error {
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

	LabelLogs                 = "logs"
	LabelLogsMisc             = "logs-misc"
	LabelLogAgent             = "log-agent"
	LabelLogGateway           = "log-gateway"
	LabelFluentBit            = "fluent-bit"
	LabelOtel                 = "otel"
	LabelOTelMaxPipeline      = "otel-max-pipeline"
	LabelFluentBitMaxPipeline = "fluent-bit-max-pipeline"
	LabelLogsMaxPipeline      = "logs-max-pipeline"

	// Metrics labels

	LabelMetrics            = "metrics"
	LabelMetricsMisc        = "metrics-misc"
	LabelMetricsMaxPipeline = "metrics-max-pipeline"
	LabelMetricAgent        = "metric-agent"
	LabelMetricAgentSetA    = "metric-agent-a"
	LabelMetricAgentSetB    = "metric-agent-b"
	LabelMetricAgentSetC    = "metric-agent-c"
	LabelMetricGateway      = "metric-gateway"
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

	LabelHealthy      = "healthy"
	LabelBackpressure = "backpressure"
	LabelOutage       = "outage"

	// LabelMisc defines the label for miscellaneous tests (for edge-cases and unrelated tests)
	// [please avoid adding tests to this category if it already fits in a more specific one]
	LabelMisc = "misc"

	// LabelIstio defines the label for Istio Integration tests
	LabelIstio = "istio"

	// LabelNoFIPS defines the label for tests that should NOT run in FIPS mode.
	// By default, all tests run with FIPS enabled. Use this label to disable FIPS
	// for specific tests (e.g., fluent-bit tests which don't support FIPS).
	LabelNoFIPS = "no-fips"

	// LabelGardener defines the label for Gardener Integration tests
	LabelGardener = "gardener"

	// LabelUpgrade defines the label for Upgrade tests. These tests start with an older
	// version of the telemetry module (deployed from UPGRADE_FROM_CHART) and then upgrade
	// to the current version mid-test using UpgradeToTargetVersion().
	LabelUpgrade = "upgrade"

	// LabelOAuth2 defines the label for OAuth2 related tests.
	LabelOAuth2 = "oauth2"
	// LabelMTLS defines the label for mTLS related tests.
	LabelMTLS = "mtls"

	LabelMaxPipeline = "max-pipeline"
	LabelSetA        = "set-a"
	LabelSetB        = "set-b"
	LabelSetC        = "set-c"
)

// ExpectAgent returns true if the test labels indicate an agent test.
// It checks for the presence of agent-related labels.
func ExpectAgent(labels ...string) bool {
	for _, label := range labels {
		switch label {
		case LabelMetricAgent, LabelLogAgent,
			LabelMetricAgentSetA, LabelMetricAgentSetB, LabelMetricAgentSetC:
			return true
		}
	}

	return false
}

func DebugObjectsEnabled() bool {
	debugEnv := os.Getenv("DEBUG_TEST_OBJECTS")
	return debugEnv == "1" || strings.ToLower(debugEnv) == "true"
}

// SetupTest prepares the test environment based on test labels.
// It registers Gomega matchers, evaluates label filters, and ensures the cluster
// is configured correctly for the test (e.g., Istio installed, experimental features enabled).
//
// This function should be called at the beginning of every test function.
// It always runs helm upgrade --install (idempotent) and deploys prerequisites.
//
// For options like custom helm values or chart version, use SetupTestWithOptions.
func SetupTest(t *testing.T, labels ...string) {
	SetupTestWithOptions(t, labels)
}

// SetupTestWithOptions prepares the test environment with additional options.
// Options can be passed to customize the setup:
//   - kubeprep.WithHelmValues("key=value") - adds custom helm values
//   - kubeprep.WithChartVersion("url") - uses a specific chart version (for upgrade tests)
func SetupTestWithOptions(t *testing.T, labels []string, opts ...kubeprep.Option) {
	RegisterTestingT(t)

	labelSet := toSet(labels)

	// Skip test if it contains "skipped" label
	if _, exists := labelSet[LabelSkip]; exists {
		t.Skip()
	}

	// Evaluate skip/execute decision BEFORE cluster configuration
	// This avoids unnecessary cluster reconfiguration for tests that won't run
	labelFilterExpr := findLabelFilterExpression()
	doNotExecute := findDoNotExecuteFlag()
	printLabels := findPrintLabelsFlag()

	// Debug log the label filter expression
	if labelFilterExpr != "" {
		t.Logf("Label filter expression: %q, test labels: %v", labelFilterExpr, labels)
	}

	// Determine if this test should run based on label filter
	shouldRun := true

	if labelFilterExpr != "" {
		var err error

		shouldRun, err = evaluateLabelExpression(labels, labelFilterExpr)
		require.NoError(t, err)
	}

	// Handle print-labels mode - print structured label info and skip
	if printLabels {
		// Only print if test would run (respects label filter)
		if shouldRun {
			printLabelsInfo(t, labels)
		}

		t.Skip()

		return
	}

	// Handle dry-run mode
	if doNotExecute {
		switch {
		case labelFilterExpr == "":
			printTestInfo(t, labels, "would execute (no filter)")
		case shouldRun:
			printTestInfo(t, labels, fmt.Sprintf("would execute (matches filter: %s)", labelFilterExpr))
		default:
			printTestInfo(t, labels, fmt.Sprintf("would skip (doesn't match filter: %s)", labelFilterExpr))
		}

		t.Skip()

		return
	}

	// Skip test if label filter doesn't match
	if !shouldRun {
		t.Skipf("Test skipped: label filter '%s' not satisfied", labelFilterExpr)
		return
	}

	// Test will execute - build configuration from labels and options
	cfg := buildConfig(labels, opts...)

	// Setup cluster (idempotent: always runs helm upgrade + prerequisites)
	require.NoError(t, kubeprep.SetupCluster(t, K8sClient, cfg))
}

// RegisterTestCase is an alias for SetupTest for backward compatibility.
//
// Deprecated: Use SetupTest instead.
func RegisterTestCase(t *testing.T, labels ...string) {
	SetupTest(t, labels...)
}

// buildConfig creates a Config from labels and applies options
func buildConfig(labels []string, opts ...kubeprep.Option) kubeprep.Config {
	// FIPS mode is enabled by default unless LabelNoFIPS is present in labels
	fipsEnabled := !hasLabel(labels, LabelNoFIPS)

	cfg := kubeprep.Config{
		OperateInFIPSMode:   fipsEnabled,
		EnableExperimental:  hasLabel(labels, LabelExperimental),
		InstallIstio:        hasLabel(labels, LabelIstio),
		DeployPrerequisites: true, // Default to deploying prerequisites
	}

	// Get manager image from environment or default
	managerImage := os.Getenv("MANAGER_IMAGE")
	if managerImage == "" {
		managerImage = DefaultLocalImage
	}

	cfg.ManagerImage = managerImage
	cfg.LocalImage = kubeprep.IsLocalImage(managerImage)

	// Apply options
	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

// UpgradeToTargetVersion upgrades the manager from a previously deployed version
// to the target version (specified by MANAGER_IMAGE, or local image if not set).
//
// This function is called mid-test in upgrade tests after validating the old version works.
// It preserves existing pipeline resources and CRDs.
func UpgradeToTargetVersion(t *testing.T, labels []string) error {
	targetImage := os.Getenv("MANAGER_IMAGE")
	if targetImage == "" {
		targetImage = DefaultLocalImage
	}

	// Build config from labels (same settings as initial setup)
	cfg := buildConfig(labels)
	cfg.ManagerImage = targetImage
	cfg.LocalImage = kubeprep.IsLocalImage(targetImage)
	cfg.ChartPath = "" // Use local chart for upgrade

	t.Logf("Upgrading manager to target version: %s (fips=%t, experimental=%t)",
		targetImage, cfg.OperateInFIPSMode, cfg.EnableExperimental)

	return kubeprep.UpgradeManagerInPlace(t, K8sClient, targetImage, cfg)
}

func findDoNotExecuteFlag() bool {
	// Ensure flags are parsed
	if !flag.Parsed() {
		flag.Parse()
	}

	return doNotExecuteFlag
}

func findPrintLabelsFlag() bool {
	// Ensure flags are parsed
	if !flag.Parsed() {
		flag.Parse()
	}

	return printLabelsFlag
}

// classifyLabels separates labels into environment-affecting and other labels
func classifyLabels(labels []string) (envLabels, otherLabels []string) {
	for _, label := range labels {
		if environmentLabels[label] {
			envLabels = append(envLabels, label)
		} else {
			otherLabels = append(otherLabels, label)
		}
	}

	return envLabels, otherLabels
}

// hasLabel checks if a specific label is present in the labels slice
func hasLabel(labels []string, target string) bool {
	return slices.Contains(labels, target)
}

// printLabelsInfo prints test labels in a structured pipe-separated format
// Format: testcase | istio | experimental | fips | env_labels | other_labels
func printLabelsInfo(t *testing.T, labels []string) {
	t.Helper()
	testName := t.Name()

	if testName == "" {
		if pc, _, _, ok := runtime.Caller(2); ok {
			if fn := runtime.FuncForPC(pc); fn != nil {
				testName = fn.Name()
				if parts := strings.Split(testName, "."); len(parts) > 0 {
					testName = parts[len(parts)-1]
				}
			}
		}

		if testName == "" {
			testName = "<unknown>"
		}
	}

	// Determine yes/no for environment labels
	istio := "no"
	if hasLabel(labels, LabelIstio) {
		istio = "yes"
	}

	experimental := "no"
	if hasLabel(labels, LabelExperimental) {
		experimental = "yes"
	}

	fips := "yes"
	if hasLabel(labels, LabelNoFIPS) {
		fips = "no"
	}

	// Classify labels
	envLabels, otherLabels := classifyLabels(labels)

	// Print in pipe-separated format
	fmt.Printf("%s | %s | %s | %s | %s | %s\n", //nolint:forbidigo // structured output for tooling
		testName,
		istio,
		experimental,
		fips,
		strings.Join(envLabels, ","),
		strings.Join(otherLabels, ","))
}

func printTestInfo(t *testing.T, labels []string, action string) {
	t.Helper()
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
	// Ensure flags are parsed
	if !flag.Parsed() {
		flag.Parse()
	}

	return labelFilterFlag
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
