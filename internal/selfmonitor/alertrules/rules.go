package alertrules

import (
	"strings"
	"time"

	"github.com/prometheus/common/model"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

const (
	// OTEL Collector rule names. Note that the actual full names will be prefixed with Metric or Trace
	RuleNameGatewayExporterSentData        = "GatewayExporterSentData"
	RuleNameGatewayExporterDroppedData     = "GatewayExporterDroppedData"
	RuleNameGatewayExporterQueueAlmostFull = "GatewayExporterQueueAlmostFull"
	RuleNameGatewayExporterEnqueueFailed   = "GatewayExporterEnqueueFailed"
	RuleNameGatewayReceiverRefusedData     = "GatewayReceiverRefusedData"

	// Fluent Bit rule names. Note that the actual full names will be prefixed with Log
	RuleNameLogAgentExporterSentLogs    = "AgentExporterSentLogs"
	RuleNameLogAgentReceiverReadLogs    = "AgentReceiverReadLogs"
	RuleNameLogAgentExporterDroppedLogs = "AgentExporterDroppedLogs"
	RuleNameLogAgentBufferInUse         = "AgentBufferInUse"
	RuleNameLogAgentBufferFull          = "AgentBufferFull"

	// Common rule labels
	labelService = "service"

	// OTel Collector rule labels
	labelExporter = "exporter"
	labelReceiver = "receiver"

	// Fluent Bit rule labels
	labelName = "name"
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

type pipelineType int

const (
	typeMetricPipeline pipelineType = iota
	typeTracePipeline
	typeLogPipeline
)

func MakeRules() RuleGroups {
	var rules []Rule

	metricRuleBuilder := otelCollectorRuleBuilder{
		dataType:    "metric_points",
		serviceName: "telemetry-metric-gateway-metrics",
		namePrefix:  ruleNamePrefix(typeMetricPipeline),
	}
	rules = append(rules, metricRuleBuilder.rules()...)

	traceRuleBuilder := otelCollectorRuleBuilder{
		dataType:    "spans",
		serviceName: "telemetry-trace-collector-metrics",
		namePrefix:  ruleNamePrefix(typeTracePipeline),
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

func ruleNamePrefix(t pipelineType) string {
	switch t {
	case typeMetricPipeline:
		return "Metric"
	case typeTracePipeline:
		return "Trace"
	case typeLogPipeline:
		return "Log"
	}
	return ""
}

const (
	RulesAny = "any"
)

// MatchesLogPipelineRule checks if the given alert label set matches the expected rule name and pipeline name for a log pipeline.
// If the alert does not have a name label, it should be matched by all pipelines.
// RulesAny can be used to match any LogPipeline rule name.
func MatchesLogPipelineRule(labelSet map[string]string, ruleNameWithoutPrefix string, pipelineName string) bool {
	if !matchesRuleName(labelSet, ruleNameWithoutPrefix, typeLogPipeline) {
		return false
	}

	nameLabel, hasNameLabel := labelSet[labelName]
	if !hasNameLabel {
		// If the alert does not have a name label, it should be matched by all pipelines
		return true
	}

	// Some Fluent Bit metrics have output plugin name appended to the pipeline name as the "name" label
	// Note that it's not possible to match a custom output since the suffix is not known in advance
	nameSuffix := "-http"
	return nameLabel == pipelineName || nameLabel == pipelineName+nameSuffix
}

// MatchesMetricPipelineRule checks if the given alert label set matches the expected rule name (or RulesAny) and pipeline name for a metric pipeline.
// If the alert does not have an exporter label, it should be matched by all pipelines.
func MatchesMetricPipelineRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string) bool {
	return matchesOTelPipelineRule(labelSet, unprefixedRuleName, pipelineName, typeMetricPipeline)
}

// MatchesTracePipelineRule checks if the given alert label set matches the expected rule name (or RulesAny) and pipeline name for a trace pipeline.
// If the alert does not have an exporter label, it should be matched by all pipelines.
func MatchesTracePipelineRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string) bool {
	return matchesOTelPipelineRule(labelSet, unprefixedRuleName, pipelineName, typeTracePipeline)
}

func matchesOTelPipelineRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string, t pipelineType) bool {
	if !matchesRuleName(labelSet, unprefixedRuleName, t) {
		return false
	}

	exporterLabel, hasExporterLabel := labelSet[labelExporter]
	if !hasExporterLabel {
		// If the alert does not have an exporter label, it should be matched by all pipelines
		return true
	}

	return otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolHTTP, pipelineName) == exporterLabel ||
		otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolGRPC, pipelineName) == exporterLabel
}

func matchesRuleName(labelSet map[string]string, unprefixedRuleName string, t pipelineType) bool {
	ruleName, hasRuleName := labelSet[model.AlertNameLabel]
	if !hasRuleName {
		return false
	}

	if !strings.HasPrefix(ruleName, ruleNamePrefix(t)) {
		return false
	}

	if unprefixedRuleName != RulesAny {
		if ruleName != ruleNamePrefix(t)+unprefixedRuleName {
			return false
		}
	}

	return true
}
