package alertrules

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/prometheus/common/model"
	"strings"
	"time"
)

const (
	// OTEL Collector rule names. Note that the actual full names will be prefixed with Metric or Trace
	RuleNameGatewayExporterSentData        = "GatewayExporterSentData"
	RuleNameGatewayExporterDroppedData     = "GatewayExporterDroppedData"
	RuleNameGatewayExporterQueueAlmostFull = "GatewayExporterQueueAlmostFull"
	RuleNameGatewayExporterEnqueueFailed   = "GatewayExporterEnqueueFailed"
	RuleNameGatewayReceiverRefusedData     = "GatewayReceiverRefusedData"

	// Fluent Bit rule names
	RuleNameLogAgentExporterSentLogs    = "LogAgentExporterSentLogs"
	RuleNameLogAgentReceiverReadLogs    = "LogAgentReceiverReadLogs"
	RuleNameLogAgentExporterDroppedLogs = "LogAgentExporterDroppedLogs"
	RuleNameLogAgentBufferInUse         = "LogAgentBufferInUse"
	RuleNameLogAgentBufferFull          = "LogAgentBufferFull"

	LabelService = "service"

	// OTel Collector rule labels
	LabelExporter = "exporter"
	LabelReceiver = "receiver"

	// Fluent Bit rule labels
	LabelName = "name"
)

// RuleGroups is a set of rule groups that are typically exposed in a file.
type RuleGroups struct {
	Groups []RuleGroup `yaml:"groups"`
}

// RuleGroup is a list of sequentially evaluated alerting rules.
type RuleGroup struct {
	Name  string `yaml:"name"`
	Rules []Rule `yaml:"rules"`
}

// Rule describes an alerting rule.
type Rule struct {
	Alert       string            `yaml:"alert,omitempty"`
	Expr        string            `yaml:"expr"`
	For         time.Duration     `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

func MakeRules() RuleGroups {
	var rules []Rule

	metricRuleBuilder := otelCollectorRuleBuilder{
		dataType:    "metric_points",
		serviceName: "telemetry-metric-gateway-metrics",
		namePrefix:  RuleNamePrefix(MetricPipeline),
	}
	rules = append(rules, metricRuleBuilder.rules()...)

	traceRuleBuilder := otelCollectorRuleBuilder{
		dataType:    "spans",
		serviceName: "telemetry-trace-collector-metrics",
		namePrefix:  RuleNamePrefix(TracePipeline),
	}
	rules = append(rules, traceRuleBuilder.rules()...)

	logRuleBuilder := fluentBitRuleBuilder{}
	rules = append(rules, logRuleBuilder.rules()...)

	return RuleGroups{
		Groups: []RuleGroup{
			{
				Name:  "default",
				Rules: rules,
			},
		},
	}
}

func RuleNamePrefix(t PipelineType) string {
	switch t {
	case MetricPipeline:
		return "Metric"
	case TracePipeline:
		return "Trace"
	case LogPipeline:
		return "Log"
	}
	return ""
}

const (
	RulesAny = "any"
)

// MatchesLogPipelineRule checks if the given alert label set matches the expected rule name (or RulesAny) and pipeline name for a log pipeline.
// If the alert does not have a name label, it should be matched by all pipelines.
func MatchesLogPipelineRule(labelSet map[string]string, expectedRuleName string, expectedPipelineName string) bool {
	ruleName, hasRuleName := labelSet[model.AlertNameLabel]
	if expectedRuleName != RulesAny {
		if !hasRuleName || ruleName != expectedRuleName {
			return false
		}
	} else {
		if !strings.HasPrefix(ruleName, RuleNamePrefix(LogPipeline)) {
			return false
		}
	}

	name, hasName := labelSet[LabelName]
	if !hasName {
		// If the alert does not have a name label, it should be matched by all pipelines
		return true
	}

	return name == expectedPipelineName
}

// MatchesMetricPipelineRule checks if the given alert label set matches the expected rule name (or RulesAny) and pipeline name for a metric pipeline.
// If the alert does not have an exporter label, it should be matched by all pipelines.
func MatchesMetricPipelineRule(labelSet map[string]string, expectedRuleName string, expectedPipelineName string) bool {
	return matchesOTelPipelineRule(labelSet, expectedRuleName, expectedPipelineName, MetricPipeline)
}

// MatchesTracePipelineRule checks if the given alert label set matches the expected rule name (or RulesAny) and pipeline name for a trace pipeline.
// If the alert does not have an exporter label, it should be matched by all pipelines.
func MatchesTracePipelineRule(labelSet map[string]string, expectedRuleName string, expectedPipelineName string) bool {
	return matchesOTelPipelineRule(labelSet, expectedRuleName, expectedPipelineName, TracePipeline)
}

func matchesOTelPipelineRule(labelSet map[string]string, expectedRuleName string, expectedPipelineName string, t PipelineType) bool {
	ruleName, hasRuleName := labelSet[model.AlertNameLabel]
	if !hasRuleName {
		return false
	}

	if expectedRuleName != RulesAny {
		expectedFullName := RuleNamePrefix(t) + expectedRuleName
		if ruleName != expectedFullName {
			return false
		}
	} else {
		if !strings.HasPrefix(ruleName, RuleNamePrefix(t)) {
			return false
		}
	}

	exporterID, hasExporter := labelSet[LabelExporter]
	if !hasExporter {
		// If the alert does not have an exporter label, it should be matched by all pipelines
		return true
	}

	return otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolHTTP, expectedPipelineName) == exporterID ||
		otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolGRPC, expectedPipelineName) == exporterID
}
