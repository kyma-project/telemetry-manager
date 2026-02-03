package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/common/model"

	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

const (
	// OTel Collector rule names for gateways. Note that the actual full names will be prefixed with Metric, or Trace, or Log

	RuleNameGatewayAllDataDropped  = "GatewayAllDataDropped"
	RuleNameGatewaySomeDataDropped = "GatewaySomeDataDropped"
	RuleNameGatewayThrottling      = "GatewayThrottling"

	// OTel Collector rule names for agents. Note that the actual full names will be prefixed with Log

	RuleNameAgentAllDataDropped  = "AgentAllDataDropped"
	RuleNameAgentSomeDataDropped = "AgentSomeDataDropped"

	// Fluent Bit rule names. Note that the actual full names will be prefixed with Log

	RuleNameLogFluentBitAllDataDropped  = "FluentBitAllDataDropped"
	RuleNameLogFluentBitSomeDataDropped = "FluentBitSomeDataDropped"
	RuleNameLogFluentBitBufferInUse     = "FluentBitBufferInUse"
	RuleNameLogFluentBitNoLogsDelivered = "FluentBitNoLogsDelivered"

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
)

func MakeRules() RuleGroups {
	var rules []Rule

	metricGatewayRuleBuilder := otelCollectorRuleBuilder{
		dataType:    ruleDataType(typeMetricPipeline),
		serviceName: names.MetricGatewayMetricsService,
		namePrefix:  ruleNamePrefix(typeMetricPipeline),
	}
	rules = append(rules, metricGatewayRuleBuilder.gatewayRules()...)

	metricAgentRuleBuilder := otelCollectorRuleBuilder{
		dataType:    ruleDataType(typeMetricPipeline),
		serviceName: names.MetricAgentMetricsService,
		namePrefix:  ruleNamePrefix(typeMetricPipeline),
	}
	rules = append(rules, metricAgentRuleBuilder.agentRules()...)

	traceGatewayRuleBuilder := otelCollectorRuleBuilder{
		dataType:    ruleDataType(typeTracePipeline),
		serviceName: names.TraceGatewayMetricsService,
		namePrefix:  ruleNamePrefix(typeTracePipeline),
	}
	rules = append(rules, traceGatewayRuleBuilder.gatewayRules()...)

	logGatewayRuleBuilder := otelCollectorRuleBuilder{
		dataType:    ruleDataType(typeLogPipeline),
		serviceName: names.LogGatewayMetricsService,
		namePrefix:  ruleNamePrefix(typeLogPipeline),
	}

	rules = append(rules, logGatewayRuleBuilder.gatewayRules()...)

	logAgentRuleBuilder := otelCollectorRuleBuilder{
		dataType:    ruleDataType(typeLogPipeline),
		serviceName: names.LogAgentMetricsService,
		namePrefix:  ruleNamePrefix(typeLogPipeline),
	}

	rules = append(rules, logAgentRuleBuilder.agentRules()...)

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

func ruleDataType(t pipelineType) string {
	var dataTypeSuffix string

	switch t {
	case typeMetricPipeline:
		dataTypeSuffix = "metric_points"
	case typeTracePipeline:
		dataTypeSuffix = "spans"
	case typeLogPipeline:
		dataTypeSuffix = "log_records"
	}

	return fmt.Sprintf("%s_total", dataTypeSuffix)
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

// MatchesMetricPipelineRule checks if the given alert label set matches the expected rule name (or RulesAny) and pipeline name for a metric pipeline.
// If the alert does not have an exporter label, it should be matched by all pipelines.
func MatchesMetricPipelineRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string) bool {
	return matchesRule(labelSet, unprefixedRuleName, pipelineName, typeMetricPipeline)
}

// MatchesTracePipelineRule checks if the given alert label set matches the expected rule name (or RulesAny) and pipeline name for a trace pipeline.
// If the alert does not have an exporter label, it should be matched by all pipelines.
func MatchesTracePipelineRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string) bool {
	return matchesRule(labelSet, unprefixedRuleName, pipelineName, typeTracePipeline)
}

// MatchesLogPipelineRule checks if the given alert label set matches the expected rule name (or RulesAny) and pipeline name for a log pipeline.
// If the alert does not have an exporter label, it should be matched by all pipelines.
func MatchesLogPipelineRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string) bool {
	return matchesRule(labelSet, unprefixedRuleName, pipelineName, typeLogPipeline)
}

func matchesRule(labelSet map[string]string, unprefixedRuleName string, pipelineName string, t pipelineType) bool {
	if !matchesRuleName(labelSet, unprefixedRuleName, t) {
		return false
	}

	pipelineNameLabel, hasLabel := labelSet[labelPipelineName]
	if !hasLabel {
		// If the alert does not have an exporter label, it should be matched by all pipelines
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
