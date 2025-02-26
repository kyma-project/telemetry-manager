package config

import (
	"strings"
	"time"

	"github.com/prometheus/common/model"

	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

const (
	// OTEL Collector rule names. Note that the actual full names will be prefixed with:
	// -  Metric/Trace/Log
	// -  Gateway/Agent
	RuleNameAllDataDropped  = "AllDataDropped"
	RuleNameSomeDataDropped = "SomeDataDropped"
	RuleNameQueueAlmostFull = "QueueAlmostFull"
	RuleNameThrottling      = "Throttling"

	// Fluent Bit rule names. Note that the actual full names will be prefixed with FluentBitLogAgent
	RuleNameFluentBitLogAgentAllDataDropped  = "AllDataDropped"
	RuleNameFluentBitLogAgentSomeDataDropped = "SomeDataDropped"
	RuleNameFluentBitLogAgentBufferInUse     = "BufferInUse"
	RuleNameFluentBitLogAgentBufferFull      = "BufferFull"
	RuleNameFluentBitLogAgentNoLogsDelivered = "NoLogsDelivered"

	// Common rule labels
	labelService      = "service"
	labelPipelineName = "pipeline_name"

	// OTel Collector rule labels
	labelReceiver = "receiver"
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
	typeFluentBitLogPipeline
)

type componentType int

const (
	componentGateway componentType = iota
	componentAgent
)

func MakeRules() RuleGroups {
	var rules []Rule

	metricRuleBuilder := otelCollectorRuleBuilder{
		dataType:    "metric_points",
		serviceName: otelcollector.MetricGatewayName + "-metrics",
		namePrefix:  ruleNamePrefix(typeMetricPipeline, componentGateway),
	}
	rules = append(rules, metricRuleBuilder.rules()...)

	traceRuleBuilder := otelCollectorRuleBuilder{
		dataType:    "spans",
		serviceName: otelcollector.TraceGatewayName + "-metrics",
		namePrefix:  ruleNamePrefix(typeTracePipeline, componentGateway),
	}
	rules = append(rules, traceRuleBuilder.rules()...)

	logRuleBuilderGateway := otelCollectorRuleBuilder{
		dataType:    "log_records",
		serviceName: otelcollector.LogGatewayName + "-metrics",
		namePrefix:  ruleNamePrefix(typeLogPipeline, componentGateway),
	}
	logRuleBuilderAgent := otelCollectorRuleBuilder{
		dataType:     "log_records",
		serviceName:  otelcollector.LogAgentName + "-metrics",
		namePrefix:   ruleNamePrefix(typeLogPipeline, componentAgent),
		excludeRules: []string{RuleNameQueueAlmostFull},
	}

	rules = append(rules, logRuleBuilderGateway.rules()...)
	rules = append(rules, logRuleBuilderAgent.rules()...)

	FluentBitLogRuleBuilder := fluentBitRuleBuilder{}
	rules = append(rules, FluentBitLogRuleBuilder.rules()...)

	return RuleGroups{
		Groups: []RuleGroup{
			{
				Name:  "default",
				Rules: rules,
			},
		},
	}
}

func ruleNamePrefix(pt pipelineType, ct componentType) string {
	ptPrefix := ""

	switch pt {
	case typeMetricPipeline:
		ptPrefix += "Metric"
	case typeTracePipeline:
		ptPrefix += "Trace"
	case typeLogPipeline:
		ptPrefix += "Log"
	case typeFluentBitLogPipeline:
		ptPrefix += "FluentBitLog"
	}

	ctPrefix := ""

	switch ct {
	case componentGateway:
		ctPrefix += "Gateway"
	case componentAgent:
		ctPrefix += "Agent"
	}

	return ptPrefix + ctPrefix
}

const (
	RulesAny = "any"
)

// MatchesFluentBitLogPipelineRule checks if the given alert label set matches the expected rule name and pipeline name for a FluentBit log pipeline.
// If the alert does not have a name label, it should be matched by all pipelines.
// RulesAny can be used to match any LogPipeline rule name.
func MatchesFluentBitLogPipelineRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string) bool {
	return matchesRule(labelSet, unprefixedRuleName, pipelineName, typeFluentBitLogPipeline, componentAgent)
}

// MatchesMetricPipelineRule checks if the given alert label set matches the expected rule name (or RulesAny) and pipeline name for a metric pipeline.
// If the alert does not have an exporter label, it should be matched by all pipelines.
func MatchesMetricPipelineRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string) bool {
	return matchesRule(labelSet, unprefixedRuleName, pipelineName, typeMetricPipeline, componentGateway)
}

// MatchesTracePipelineRule checks if the given alert label set matches the expected rule name (or RulesAny) and pipeline name for a trace pipeline.
// If the alert does not have an exporter label, it should be matched by all pipelines.
func MatchesTracePipelineRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string) bool {
	return matchesRule(labelSet, unprefixedRuleName, pipelineName, typeTracePipeline, componentGateway)
}

// MatchesLogPipelineRule checks if the given alert label set matches the expected rule name (or RulesAny) and pipeline name for a log pipeline.
// If the alert does not have an exporter label, it should be matched by all pipelines.
func MatchesLogPipelineRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string) bool {
	matchesAgentRule := matchesRule(labelSet, unprefixedRuleName, pipelineName, typeLogPipeline, componentAgent)
	matchesGatewayRule := matchesRule(labelSet, unprefixedRuleName, pipelineName, typeLogPipeline, componentGateway)

	return matchesAgentRule || matchesGatewayRule
}

func matchesRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string, pt pipelineType, ct componentType) bool {
	if !matchesRuleName(labelSet, unprefixedRuleName, pt, ct) {
		return false
	}

	pipelineNameLabel, hasLabel := labelSet[labelPipelineName]
	if !hasLabel {
		// If the alert does not have an exporter label, it should be matched by all pipelines
		return true
	}

	return pipelineNameLabel == pipelineName
}

func matchesRuleName(labelSet map[string]string, unprefixedRuleName string, pt pipelineType, ct componentType) bool {
	ruleName, hasRuleName := labelSet[model.AlertNameLabel]
	if !hasRuleName {
		return false
	}

	if !strings.HasPrefix(ruleName, ruleNamePrefix(pt, ct)) {
		return false
	}

	if unprefixedRuleName != RulesAny {
		if ruleName != ruleNamePrefix(pt, ct)+unprefixedRuleName {
			return false
		}
	}

	return true
}
