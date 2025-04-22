package config

import (
	"time"
)

const (
	fluentBitMetricsServiceName = "telemetry-fluent-bit-metrics"

	// Fluent Bit metrics
	fluentBitOutputProcBytesTotal      = "fluentbit_output_proc_bytes_total"
	fluentBitInputBytesTotal           = "fluentbit_input_bytes_total"
	fluentBitOutputDroppedRecordsTotal = "fluentbit_output_dropped_records_total"
	fluentBitInputStorageChunksDown    = "fluentbit_input_storage_chunks_down"

	inputStorageChunksDown300Chunks = 300

	// alertWaitTime is the time the alert have a pending state before firing
	alertWaitTime = 1 * time.Minute
)

type fluentBitRuleBuilder struct {
}

func (rb fluentBitRuleBuilder) rules() []Rule {
	return []Rule{
		rb.makeRule(RuleNameLogFluentBitAllDataDropped, rb.allDataDroppedExpr()),
		rb.makeRule(RuleNameLogFluentBitSomeDataDropped, rb.someDataDroppedExpr()),
		rb.makeRule(RuleNameLogFluentBitBufferInUse, rb.bufferInUseExpr()),
		rb.makeRule(RuleNameLogFluentBitNoLogsDelivered, rb.noLogsDeliveredExpr()),
	}
}

// Checks if all data is dropped due to exporter issues, with nothing successfully sent.
func (rb fluentBitRuleBuilder) allDataDroppedExpr() string {
	return unless(rb.exporterDroppedExpr(), rb.exporterSentExpr())
}

// Checks if some data is dropped while some is still successfully sent.
func (rb fluentBitRuleBuilder) someDataDroppedExpr() string {
	return and(rb.exporterDroppedExpr(), rb.exporterSentExpr())
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
	return instant(fluentBitInputStorageChunksDown, selectService(fluentBitMetricsServiceName)).
		maxBy(labelPipelineName).
		greaterThan(inputStorageChunksDown300Chunks).
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
