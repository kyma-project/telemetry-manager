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

// FIPSImagesAvailable returns true if FIPS images are accessible in the current environment.
// This is determined by the FIPS_IMAGE_AVAILABLE environment variable.
// In CI: true on push (has registry access), false on PR (no registry access).
func FIPSImagesAvailable() bool {
	env := os.Getenv("FIPS_IMAGE_AVAILABLE")
	return env == "1" || strings.ToLower(env) == "true"
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
//   - kubeprep.WithIstio() - installs Istio and adds LabelIstio for filtering
//   - kubeprep.WithExperimental() - enables experimental CRDs and adds LabelExperimental for filtering
//   - kubeprep.WithHelmValues("key=value") - adds custom helm values
//   - kubeprep.WithChartVersion("url") - uses a specific chart version (for upgrade tests)
//   - kubeprep.WithOverrideFIPSMode(bool) - overrides FIPS mode setting
func SetupTestWithOptions(t *testing.T, labels []string, opts ...kubeprep.Option) {
	RegisterTestingT(t)

	// Build config from options
	cfg := buildConfig(opts...)

	// Auto-add labels based on config values (options → labels)
	labels = addLabelsFromConfig(labels, cfg)

	// Skip test if it contains "skipped" label
	if hasLabel(labels, LabelSkip) {
		t.Skip()
	}

	// Check if test should run based on filters and special modes
	if handleTestFiltering(t, labels) {
		return // test was skipped
	}

	// Log FIPS configuration for clarity
	logFIPSConfiguration(t, cfg)

	// Setup cluster (idempotent: always runs helm upgrade + prerequisites)
	require.NoError(t, kubeprep.SetupCluster(t, K8sClient, cfg))
}

// addLabelsFromConfig auto-adds labels based on config values
// This ensures label filtering still works when using options
func addLabelsFromConfig(labels []string, cfg kubeprep.Config) []string {
	if cfg.InstallIstio && !hasLabel(labels, LabelIstio) {
		labels = append(labels, LabelIstio)
	}

	if cfg.EnableExperimental && !hasLabel(labels, LabelExperimental) {
		labels = append(labels, LabelExperimental)
	}

	return labels
}

// handleTestFiltering handles label filtering, dry-run mode, and print-labels mode.
// Returns true if the test should be skipped (already handled), false if it should proceed.
func handleTestFiltering(t *testing.T, labels []string) bool {
	t.Helper()

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
		if shouldRun {
			printLabelsInfo(t, labels)
		}

		t.Skip()

		return true
	}

	// Handle dry-run mode
	if doNotExecute {
		handleDryRunMode(t, labels, labelFilterExpr, shouldRun)
		return true
	}

	// Skip test if label filter doesn't match
	if !shouldRun {
		t.Skipf("Test skipped: label filter '%s' not satisfied", labelFilterExpr)
		return true
	}

	return false
}

// handleDryRunMode prints test info in dry-run mode
func handleDryRunMode(t *testing.T, labels []string, labelFilterExpr string, shouldRun bool) {
	t.Helper()

	switch {
	case labelFilterExpr == "":
		printTestInfo(t, labels, "would execute (no filter)")
	case shouldRun:
		printTestInfo(t, labels, fmt.Sprintf("would execute (matches filter: %s)", labelFilterExpr))
	default:
		printTestInfo(t, labels, fmt.Sprintf("would skip (doesn't match filter: %s)", labelFilterExpr))
	}

	t.Skip()
}

// finalizeConfig completes the config with manager image information.
// The config should already have InstallIstio, EnableExperimental, etc. set by options.
func finalizeConfig(cfg kubeprep.Config) kubeprep.Config {
	// Get manager image from environment or default
	managerImage := os.Getenv("MANAGER_IMAGE")
	if managerImage == "" {
		managerImage = DefaultLocalImage
	}

	cfg.ManagerImage = managerImage
	cfg.LocalImage = kubeprep.IsLocalImage(managerImage)

	return cfg
}

// buildConfig creates a Config from options only.
// Labels are used solely for test filtering, not for configuration.
// Configuration must be explicitly set via functional options.
func buildConfig(opts ...kubeprep.Option) kubeprep.Config {
	// FIPS mode default is determined by environment (FIPS_IMAGE_AVAILABLE).
	// WithOverrideFIPSMode() option can override this for specific tests.
	fipsEnabled := FIPSImagesAvailable()

	cfg := kubeprep.Config{
		OperateInFIPSMode:   fipsEnabled,
		DeployPrerequisites: true, // Default to deploying prerequisites
	}

	// Apply options to configure the test environment
	for _, opt := range opts {
		opt(&cfg)
	}

	return finalizeConfig(cfg)
}

// logFIPSConfiguration logs the FIPS mode configuration for clarity
func logFIPSConfiguration(t *testing.T, cfg kubeprep.Config) {
	t.Helper()

	fipsImagesAvailable := FIPSImagesAvailable()

	// Determine how FIPS mode was set
	fipsModeSource := "environment default"
	if cfg.FIPSModeOverridden {
		fipsModeSource = "test override (WithOverrideFIPSMode)"
	}

	t.Logf("FIPS configuration: imagesAvailable=%t, fipsMode=%t (source: %s)",
		fipsImagesAvailable, cfg.OperateInFIPSMode, fipsModeSource)
}

// UpgradeToTargetVersion upgrades the manager from a previously deployed version
// to the target version (specified by MANAGER_IMAGE, or local image if not set).
//
// This function is called mid-test in upgrade tests after validating the old version works.
// It preserves existing pipeline resources and CRDs by using SetupCluster with
// SkipManagerRemoval enabled.
//
// Options passed to this function should match those passed to SetupTestWithOptions
// (e.g., kubeprep.WithOverrideFIPSMode(false)) to ensure consistent configuration.
func UpgradeToTargetVersion(t *testing.T, opts ...kubeprep.Option) error {
	// Add SkipManagerRemoval to preserve existing pipelines
	opts = append(opts, kubeprep.WithSkipManagerRemoval(), kubeprep.WithSkipDeployTestPrerequisites())

	// Build config from options
	cfg := buildConfig(opts...)

	t.Logf("Upgrading manager to target version: %s (fips=%t, experimental=%t)",
		cfg.ManagerImage, cfg.OperateInFIPSMode, cfg.EnableExperimental)

	return kubeprep.SetupCluster(t, K8sClient, cfg)
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

	// FIPS is determined by environment default
	fips := "no"
	if FIPSImagesAvailable() {
		fips = "yes"
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
