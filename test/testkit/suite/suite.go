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

var labelFilterFlag string

// init registers the -labels flag with the flag package.
func init() {
	flag.StringVar(&labelFilterFlag, "labels", "", "Label filter expression (e.g., 'log-agent and istio')")
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

func RegisterTestCase(t *testing.T, labels ...string) {
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

	// Determine if this test should run based on label filter
	shouldRun := true
	if labelFilterExpr != "" {
		var err error
		shouldRun, err = evaluateLabelExpression(labels, labelFilterExpr)
		require.NoError(t, err)
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

func findDoNotExecuteFlag() bool {
	for _, arg := range os.Args {
		if arg == "-do-not-execute" || arg == "--do-not-execute" {
			return true
		}
	}

	return false
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
func ensureClusterState(t *testing.T, requiredConfig kubeprep.Config) error {
	// Always detect current cluster state to handle drift between tests
	// This is important because the actual cluster state might differ from our in-memory tracking
	// (e.g., if a previous test run crashed or was interrupted)
	t.Log("Detecting current cluster state...")
	detectedState, err := kubeprep.DetectClusterState(t, K8sClient)
	if err != nil {
		return fmt.Errorf("failed to detect cluster state: %w", err)
	}
	CurrentClusterState = detectedState

	t.Logf("Detected state: NeedsReinstall=%t, EnableExperimental=%t, SkipManagerDeployment=%t",
		CurrentClusterState.NeedsReinstall, CurrentClusterState.EnableExperimental, CurrentClusterState.SkipManagerDeployment)

	// Copy immutable fields from current state (these don't change during reconfigurations)
	// ManagerImage and LocalImage are preserved from the current state
	requiredConfig.ManagerImage = CurrentClusterState.ManagerImage
	requiredConfig.LocalImage = CurrentClusterState.LocalImage

	// If ManagerImage is still empty, try to get it from environment or use default
	if requiredConfig.ManagerImage == "" {
		managerImage := os.Getenv("MANAGER_IMAGE")
		if managerImage == "" {
			managerImage = "telemetry-manager:latest"
		}
		requiredConfig.ManagerImage = managerImage
		requiredConfig.LocalImage = kubeprep.IsLocalImage(managerImage)

		// Update current state with the image info
		CurrentClusterState.ManagerImage = managerImage
		CurrentClusterState.LocalImage = requiredConfig.LocalImage
	}

	t.Logf("Required state: NeedsReinstall=%t, EnableExperimental=%t, SkipManagerDeployment=%t",
		requiredConfig.NeedsReinstall, requiredConfig.EnableExperimental, requiredConfig.SkipManagerDeployment)

	// Check if reconfiguration is needed
	if configsEqual(*CurrentClusterState, requiredConfig) {
		t.Log("Cluster state matches requirements, no reconfiguration needed")
		return nil
	}

	t.Log("Cluster state mismatch, reconfiguring...")
	t.Logf("  Current: InstallIstio=%t, OperateInFIPSMode=%t, EnableExperimental=%t, CustomLabelsAnnotations=%t, NeedsReinstall=%t",
		CurrentClusterState.InstallIstio, CurrentClusterState.OperateInFIPSMode,
		CurrentClusterState.EnableExperimental, CurrentClusterState.CustomLabelsAnnotations, CurrentClusterState.NeedsReinstall)
	t.Logf("  Required: InstallIstio=%t, OperateInFIPSMode=%t, EnableExperimental=%t, CustomLabelsAnnotations=%t, NeedsReinstall=%t",
		requiredConfig.InstallIstio, requiredConfig.OperateInFIPSMode,
		requiredConfig.EnableExperimental, requiredConfig.CustomLabelsAnnotations, requiredConfig.NeedsReinstall)

	// Apply reconfiguration using t directly (implements kubeprep.TestingT)
	if err := kubeprep.ReconfigureCluster(t, K8sClient, *CurrentClusterState, requiredConfig); err != nil {
		return fmt.Errorf("reconfiguration failed: %w", err)
	}

	// Update current state to reflect the reconfiguration
	*CurrentClusterState = requiredConfig
	t.Log("Cluster reconfigured successfully")

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
