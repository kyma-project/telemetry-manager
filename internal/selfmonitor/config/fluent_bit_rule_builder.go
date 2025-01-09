package config

import (
	"time"
)

const (
	fluentBitMetricsServiceName        = "telemetry-fluent-bit-metrics"
	fluentBitSidecarMetricsServiceName = "telemetry-fluent-bit-exporter-metrics"

	metricFluentBitOutputProcBytesTotal      = "fluentbit_output_proc_bytes_total"
	metricFluentBitInputBytesTotal           = "fluentbit_input_bytes_total"
	metricFluentBitOutputDroppedRecordsTotal = "fluentbit_output_dropped_records_total"
	metricFluentBitBufferUsageBytes          = "telemetry_fsbuffer_usage_bytes"

	bufferUsage300MB = 300000000
	bufferUsage900MB = 900000000

	// alertWaitTime is the time the alert have a pending state before firing
	alertWaitTime = 1 * time.Minute
)

type fluentBitRuleBuilder struct {
}

func (rb fluentBitRuleBuilder) rules() []Rule {
	return []Rule{
		rb.allDataDroppedRule(),
		rb.someDataDroppedRule(),
		rb.bufferInUseRule(),
		rb.bufferFullRule(),
		rb.noLogsDeliveredRule(),
	}
}

func (rb fluentBitRuleBuilder) allDataDroppedRule() Rule {
	bufferFullExpr := instant(metricFluentBitBufferUsageBytes, selectService(fluentBitSidecarMetricsServiceName)).
		greaterThan(bufferUsage900MB).
		build()

	exporterDroppedExpr := rate(metricFluentBitOutputDroppedRecordsTotal, selectService(fluentBitMetricsServiceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()

	exporterSentExpr := rate(metricFluentBitOutputProcBytesTotal, selectService(fluentBitMetricsServiceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()

	expr := unless(or(bufferFullExpr, exporterDroppedExpr), exporterSentExpr)

	return rb.makeRule(RuleNameLogAgentAllDataDropped, expr)
}

func (rb fluentBitRuleBuilder) someDataDroppedRule() Rule {
	bufferFullExpr := instant(metricFluentBitBufferUsageBytes, selectService(fluentBitSidecarMetricsServiceName)).
		greaterThan(bufferUsage900MB).
		build()

	exporterDroppedExpr := rate(metricFluentBitOutputDroppedRecordsTotal, selectService(fluentBitMetricsServiceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()

	exporterSentExpr := rate(metricFluentBitOutputProcBytesTotal, selectService(fluentBitMetricsServiceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()

	expr := and(or(bufferFullExpr, exporterDroppedExpr), exporterSentExpr)

	return rb.makeRule(RuleNameLogAgentSomeDataDropped, expr)
}

func (rb fluentBitRuleBuilder) bufferInUseRule() Rule {
	expr := instant(metricFluentBitBufferUsageBytes, selectService(fluentBitSidecarMetricsServiceName)).
		greaterThan(bufferUsage300MB).
		build()

	return rb.makeRule(RuleNameLogAgentBufferInUse, expr)
}

func (rb fluentBitRuleBuilder) bufferFullRule() Rule {
	expr := instant(metricFluentBitBufferUsageBytes, selectService(fluentBitSidecarMetricsServiceName)).
		greaterThan(bufferUsage900MB).
		build()

	return rb.makeRule(RuleNameLogAgentBufferFull, expr)
}

func (rb fluentBitRuleBuilder) noLogsDeliveredRule() Rule {
	receiverReadExpr := rate(metricFluentBitInputBytesTotal, selectService(fluentBitMetricsServiceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()

	exporterNotSentExpr := rate(metricFluentBitOutputProcBytesTotal, selectService(fluentBitMetricsServiceName)).
		sumBy(labelPipelineName).
		equal(0).
		build()

	expr := and(receiverReadExpr, exporterNotSentExpr)

	return rb.makeRule(RuleNameLogAgentNoLogsDelivered, expr)
}

func (rb fluentBitRuleBuilder) makeRule(baseName, expr string) Rule {
	return Rule{
		Alert: rb.namePrefix() + baseName,
		Expr:  expr,
		For:   alertWaitTime,
	}
}

func (rb fluentBitRuleBuilder) namePrefix() string {
	return ruleNamePrefix(typeLogPipeline)
}
