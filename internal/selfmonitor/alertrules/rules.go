package alertrules

import (
	"strings"
	"time"

	"github.com/prometheus/common/model"
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

	labelReceiver     = "receiver"
	labelPipelineName = "pipeline_name"
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

// MatchesRule checks if the given alert label set matches the expected rule name and pipeline name for a given log/trace/metric pipeline.
// If the alert does not have the pipeline_name label, it should be matched by all pipelines.
// RulesAny can be used to match any LogPipeline rule name.
func MatchesRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string) bool {
	if !matchesRuleName(labelSet, unprefixedRuleName, typeLogPipeline) {
		return false
	}

	pipelineNameLabel, hasLabel := labelSet[labelPipelineName]
	if !hasLabel {
		// If the alert does not have a name label, it should be matched by all pipelines
		return true
	}

	return pipelineNameLabel == pipelineName
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
