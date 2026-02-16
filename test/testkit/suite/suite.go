package suite

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
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

	// ClusterPrepConfig enables dynamic cluster configuration
	// Set this before calling BeforeSuiteFunc() to prepare cluster
	ClusterPrepConfig *kubeprep.Config

	// CurrentClusterState tracks the actual current cluster configuration
	// Updated after each reconfiguration to reflect the real cluster state
	CurrentClusterState *kubeprep.Config
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

// init registers test flags with the flag package.
func init() {
	flag.StringVar(&labelFilterFlag, "labels", "", "Label filter expression (e.g., 'log-agent and istio')")
	flag.BoolVar(&doNotExecuteFlag, "do-not-execute", false, "Dry-run mode: print test info without executing")
	flag.BoolVar(&printLabelsFlag, "print-labels", false, "Print test labels in structured format (pipe-separated)")
}

// BeforeSuiteFunc is designed to return an error instead of relying on Gomega matchers.
// This function is intended for use in a vanilla TestMain function within new e2e test suites.
// Note that Gomega matchers cannot be utilized in the TestMain function.
//
// This function only initializes the K8s client and context. Cluster preparation
// is handled dynamically by RegisterTestCase based on test labels.
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
)

func ExpectAgent(label string) bool {
	return label == LabelMetricAgentSetA ||
		label == LabelMetricAgentSetB ||
		label == LabelMetricAgentSetC ||
		label == LabelLogAgent
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
func SetupTest(t *testing.T, labels ...string) {
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
		if labelFilterExpr == "" {
			printTestInfo(t, labels, "would execute (no filter)")
		} else if shouldRun {
			printTestInfo(t, labels, fmt.Sprintf("would execute (matches filter: %s)", labelFilterExpr))
		} else {
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

	// Test will execute - now ensure cluster is configured correctly
	// Infer required cluster configuration from labels
	requiredConfig := InferRequirementsFromLabels(labels)

	// Reconfigure cluster if needed
	// Note: Prerequisite validation is no longer needed - reconfiguration handles it automatically
	require.NoError(t, ensureClusterState(t, requiredConfig))
}

// RegisterTestCase is an alias for SetupTest for backward compatibility.
// Deprecated: Use SetupTest instead.
func RegisterTestCase(t *testing.T, labels ...string) {
	SetupTest(t, labels...)
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
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
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

// ensureClusterState reconfigures cluster if current state doesn't match required state.
// This function is called for every test to ensure the cluster is in the correct state
// before the test runs. It always detects the current cluster state to handle drift.
//
// For upgrade tests (IsUpgradeTest=true with UpgradeFromChart set), this function
// deploys from the remote helm chart instead of the local one.
func ensureClusterState(t *testing.T, requiredConfig kubeprep.Config) error {
	// Special handling for upgrade tests with UPGRADE_FROM_CHART
	if requiredConfig.IsUpgradeTest && requiredConfig.UpgradeFromChart != "" {
		return ensureUpgradeTestState(t, requiredConfig)
	}

	// Always detect current cluster state to handle drift between tests
	detectedState, err := kubeprep.DetectClusterState(t, K8sClient)
	if err != nil {
		return fmt.Errorf("failed to detect cluster state: %w", err)
	}
	CurrentClusterState = detectedState

	// Copy immutable fields from current state (these don't change during reconfigurations)
	requiredConfig.ManagerImage = CurrentClusterState.ManagerImage
	requiredConfig.LocalImage = CurrentClusterState.LocalImage

	// If ManagerImage is still empty, try to get it from environment or use default
	if requiredConfig.ManagerImage == "" {
		managerImage := os.Getenv("MANAGER_IMAGE")
		if managerImage == "" {
			managerImage = DefaultLocalImage
		}
		requiredConfig.ManagerImage = managerImage
		requiredConfig.LocalImage = kubeprep.IsLocalImage(managerImage)

		CurrentClusterState.ManagerImage = managerImage
		CurrentClusterState.LocalImage = requiredConfig.LocalImage
	}

	// Check if reconfiguration is needed
	if configsEqual(*CurrentClusterState, requiredConfig) {
		return nil
	}

	t.Logf("Reconfiguring cluster:\ncurrent=%+v\nrequired=%+v", *CurrentClusterState, requiredConfig)

	if err := kubeprep.ReconfigureCluster(t, K8sClient, *CurrentClusterState, requiredConfig); err != nil {
		return fmt.Errorf("reconfiguration failed: %w", err)
	}

	*CurrentClusterState = requiredConfig
	return nil
}

// ensureUpgradeTestState handles cluster setup for upgrade tests.
// It deploys the manager from a remote helm chart (the old version) when UPGRADE_FROM_CHART is set.
func ensureUpgradeTestState(t *testing.T, requiredConfig kubeprep.Config) error {
	t.Logf("Setting up upgrade test from chart: %s", requiredConfig.UpgradeFromChart)

	// Detect current state to see if manager is already deployed
	detectedState, err := kubeprep.DetectClusterState(t, K8sClient)
	if err != nil {
		return fmt.Errorf("failed to detect cluster state: %w", err)
	}

	// If manager is already deployed, we need to undeploy it first
	// (upgrade tests need a clean slate with the old version)
	// SkipManagerDeployment=false means manager IS deployed
	if !detectedState.SkipManagerDeployment {
		t.Log("Manager already deployed, undeploying before upgrade test setup...")
		if err := kubeprep.ReconfigureCluster(t, K8sClient, *detectedState, kubeprep.Config{
			SkipManagerDeployment: true,
			SkipPrerequisites:     true,
			NeedsReinstall:        true,
		}); err != nil {
			return fmt.Errorf("failed to undeploy existing manager: %w", err)
		}
	}

	// Deploy from the remote chart (old version) with the required settings
	if err := kubeprep.DeployManagerFromChart(t, K8sClient, requiredConfig.UpgradeFromChart, requiredConfig); err != nil {
		return fmt.Errorf("failed to deploy manager from chart: %w", err)
	}

	// Deploy test prerequisites
	if err := kubeprep.DeployTestPrerequisitesPublic(t, K8sClient); err != nil {
		return fmt.Errorf("failed to deploy test prerequisites: %w", err)
	}

	// Update current state to reflect what we deployed
	// Store the config so UpgradeToTargetVersion can use the same settings
	CurrentClusterState = &kubeprep.Config{
		IsUpgradeTest:           true,
		UpgradeFromChart:        requiredConfig.UpgradeFromChart,
		OperateInFIPSMode:       requiredConfig.OperateInFIPSMode,
		EnableExperimental:      requiredConfig.EnableExperimental,
		CustomLabelsAnnotations: requiredConfig.CustomLabelsAnnotations,
	}

	return nil
}

// UpgradeToTargetVersion upgrades the manager from the old version (deployed via UPGRADE_FROM_CHART)
// to the target version (specified by MANAGER_IMAGE, or local image if not set).
// This function is called mid-test in upgrade tests after validating the old version works.
//
// It uses the same FIPS, experimental, and custom labels/annotations settings that were
// used for the old version to ensure consistency during upgrade.
func UpgradeToTargetVersion(t *testing.T) error {
	targetImage := os.Getenv("MANAGER_IMAGE")
	if targetImage == "" {
		// Default to local image when MANAGER_IMAGE is not set
		targetImage = DefaultLocalImage
	}

	// Get the config settings from the current state (set during old version deployment)
	cfg := kubeprep.Config{}
	if CurrentClusterState != nil {
		cfg = *CurrentClusterState
	}

	t.Logf("Upgrading manager to target version: %s (fips=%t, experimental=%t)",
		targetImage, cfg.OperateInFIPSMode, cfg.EnableExperimental)

	if err := kubeprep.UpgradeManagerInPlace(t, K8sClient, targetImage, cfg); err != nil {
		return fmt.Errorf("failed to upgrade manager: %w", err)
	}

	// Update current state
	if CurrentClusterState != nil {
		CurrentClusterState.ManagerImage = targetImage
		CurrentClusterState.LocalImage = kubeprep.IsLocalImage(targetImage)
		CurrentClusterState.IsUpgradeTest = false // No longer in "old version" state
		CurrentClusterState.UpgradeFromChart = ""
	}

	return nil
}

// configsEqual compares two configs for equality, ignoring ManagerImage and LocalImage
// which are immutable during test execution.
// If NeedsReinstall is true on either config, they are considered NOT equal to force reconfiguration.
func configsEqual(a, b kubeprep.Config) bool {
	// If either config needs reinstall, always reconfigure
	if a.NeedsReinstall || b.NeedsReinstall {
		return false
	}

	return a.InstallIstio == b.InstallIstio &&
		a.OperateInFIPSMode == b.OperateInFIPSMode &&
		a.EnableExperimental == b.EnableExperimental &&
		a.CustomLabelsAnnotations == b.CustomLabelsAnnotations &&
		a.SkipManagerDeployment == b.SkipManagerDeployment &&
		a.SkipPrerequisites == b.SkipPrerequisites
}
