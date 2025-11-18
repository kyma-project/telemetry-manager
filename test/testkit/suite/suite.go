package suite

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/expr-lang/expr"
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

// expr-lang/expr uses different syntax for logical operators. for our purpose AND OR NOT make more sense.
func convertLabelExpressionSyntax(legacyExpr string) string {
	if strings.TrimSpace(legacyExpr) == "" {
		return ""
	}

	converted := strings.ToLower(legacyExpr)

	// Replace operators with expr-lang syntax
	// Use word boundaries to avoid replacing parts of label names
	converted = replaceWord(converted, "and", "&&")
	converted = replaceWord(converted, "or", "||")
	// For NOT, we need special handling to avoid adding extra space
	converted = replaceWordCompact(converted, "not", "!")

	return converted
}

// replaceWordCompact replaces a word and removes the trailing space if present
func replaceWordCompact(s, old, new string) string {
	var result strings.Builder
	i := 0
	oldLen := len(old)

	for i < len(s) {
		// Check if we found the word at current position
		if i+oldLen <= len(s) && s[i:i+oldLen] == old {
			// Check if it's a complete word (not part of another word)
			beforeOK := i == 0 || !isAlphaNumeric(s[i-1])
			afterOK := i+oldLen == len(s) || !isAlphaNumeric(s[i+oldLen])

			if beforeOK && afterOK {
				result.WriteString(new)
				i += oldLen
				// Skip one trailing space if present
				if i < len(s) && s[i] == ' ' {
					i++
				}
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}

	return result.String()
}

// replaceWord replaces whole words only, not substrings within words
func replaceWord(s, old, new string) string {
	var result strings.Builder
	i := 0
	oldLen := len(old)

	for i < len(s) {
		// Check if we found the word at current position
		if i+oldLen <= len(s) && s[i:i+oldLen] == old {
			// Check if it's a complete word (not part of another word)
			beforeOK := i == 0 || !isAlphaNumeric(s[i-1])
			afterOK := i+oldLen == len(s) || !isAlphaNumeric(s[i+oldLen])

			if beforeOK && afterOK {
				result.WriteString(new)
				i += oldLen
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}

	return result.String()
}

func isAlphaNumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
}

// evaluateLabelExpression evaluates a label filter expression against test labels using expr-lang/expr
func evaluateLabelExpression(testLabels []string, filterExpr string) (bool, error) {
	if strings.TrimSpace(filterExpr) == "" {
		return true, nil // No filter means run all tests
	}

	// Convert legacy syntax to expr syntax
	exprSyntax := convertLabelExpressionSyntax(filterExpr)

	// Build environment - create a map that returns false for missing keys
	// This is done by using a custom type that implements map-like access
	labelSet := make(map[string]bool)
	for _, label := range testLabels {
		labelSet[strings.ToLower(label)] = true
	}

	// Create environment accessor that returns false for undefined labels
	env := map[string]interface{}{
		"hasLabel": func(label string) bool {
			return labelSet[strings.ToLower(label)]
		},
	}

	// Transform the expression to use hasLabel() function calls
	transformedExpr := transformExpressionToFunctionCalls(exprSyntax)

	// Compile and run the expression
	program, err := expr.Compile(transformedExpr, expr.Env(env), expr.AsBool())
	if err != nil {
		return false, fmt.Errorf("invalid label filter expression '%s': %w", filterExpr, err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate label filter '%s': %w", filterExpr, err)
	}

	return result.(bool), nil
}

// transformExpressionToFunctionCalls converts label identifiers to hasLabel() function calls
func transformExpressionToFunctionCalls(exprStr string) string {
	var result strings.Builder
	i := 0

	for i < len(exprStr) {
		// Skip operators and special characters
		if exprStr[i] == '&' || exprStr[i] == '|' || exprStr[i] == '!' ||
			exprStr[i] == '(' || exprStr[i] == ')' || exprStr[i] == ' ' {
			result.WriteByte(exprStr[i])
			i++
			continue
		}

		// We found the start of an identifier
		start := i
		for i < len(exprStr) && isAlphaNumeric(exprStr[i]) {
			i++
		}

		label := exprStr[start:i]
		result.WriteString(fmt.Sprintf("hasLabel(\"%s\")", label))
	}

	return result.String()
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
		set[label] = struct{}{}
	}

	return set
}
