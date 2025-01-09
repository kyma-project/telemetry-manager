package config

import (
	"time"
)

const (
	fluentBitMetricsServiceName        = "telemetry-fluent-bit-metrics"
	fluentBitSidecarMetricsServiceName = "telemetry-fluent-bit-exporter-metrics"

	//Fluent Bit metrics
	fluentBitOutputProcBytesTotal      = "fluentbit_output_proc_bytes_total"
	fluentBitInputBytesTotal           = "fluentbit_input_bytes_total"
	fluentBitOutputDroppedRecordsTotal = "fluentbit_output_dropped_records_total"
	fluentBitBufferUsageBytes          = "telemetry_fsbuffer_usage_bytes"

	bufferUsage300MB = 300000000
	bufferUsage900MB = 900000000

	// alertWaitTime is the time the alert have a pending state before firing
	alertWaitTime = 1 * time.Minute
)

type fluentBitRuleBuilder struct {
}

func (rb fluentBitRuleBuilder) rules() []Rule {
	return []Rule{
		rb.makeRule(RuleNameLogAgentAllDataDropped, rb.allDataDroppedExpr()),
		rb.makeRule(RuleNameLogAgentSomeDataDropped, rb.someDataDroppedExpr()),
		rb.makeRule(RuleNameLogAgentBufferInUse, rb.bufferInUseExpr()),
		rb.makeRule(RuleNameLogAgentBufferFull, rb.bufferFullExpr()),
		rb.makeRule(RuleNameLogAgentNoLogsDelivered, rb.noLogsDeliveredExpr()),
	}
}

// Checks if all data is dropped due to a full buffer or exporter issues, with nothing successfully sent.
func (rb fluentBitRuleBuilder) allDataDroppedExpr() string {
	return unless(
		or(rb.bufferFullExpr(), rb.exporterDroppedExpr()),
		rb.exporterSentExpr(),
	)
}

// Checks if some data is dropped while some is still successfully sent.
func (rb fluentBitRuleBuilder) someDataDroppedExpr() string {
	return and(
		or(rb.bufferFullExpr(), rb.exporterDroppedExpr()),
		rb.exporterSentExpr(),
	)
}

// Checks if the exporter drop rate is greater than 0.
func (rb fluentBitRuleBuilder) exporterDroppedExpr() string {
	return rate(fluentBitOutputDroppedRecordsTotal, selectService(fluentBitMetricsServiceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()
}

// Check if the exporter send rate is greater than 0.
func (rb fluentBitRuleBuilder) exporterSentExpr() string {
	return rate(fluentBitOutputProcBytesTotal, selectService(fluentBitMetricsServiceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()
}

// Check if the buffer usage is significant.
func (rb fluentBitRuleBuilder) bufferInUseExpr() string {
	return instant(fluentBitBufferUsageBytes, selectService(fluentBitSidecarMetricsServiceName)).
		greaterThan(bufferUsage300MB).
		build()
}

// Check if the buffer usage is approaching the limit (1GB).
func (rb fluentBitRuleBuilder) bufferFullExpr() string {
	return instant(fluentBitBufferUsageBytes, selectService(fluentBitSidecarMetricsServiceName)).
		greaterThan(bufferUsage900MB).
		build()
}

// Checks if logs are read but not sent by the exporter.
func (rb fluentBitRuleBuilder) noLogsDeliveredExpr() string {
	receiverReadExpr := rate(fluentBitInputBytesTotal, selectService(fluentBitMetricsServiceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()

	exporterNotSentExpr := rate(fluentBitOutputProcBytesTotal, selectService(fluentBitMetricsServiceName)).
		sumBy(labelPipelineName).
		equal(0).
		build()

	return and(receiverReadExpr, exporterNotSentExpr)
}

func (rb fluentBitRuleBuilder) makeRule(baseName, expr string) Rule {
	return Rule{
		Alert: ruleNamePrefix(typeLogPipeline) + baseName,
		Expr:  expr,
		For:   alertWaitTime,
	}
}
