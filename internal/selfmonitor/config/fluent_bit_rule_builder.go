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
		rb.exporterSentRule(),
		rb.exporterDroppedRule(),
		rb.bufferInUseRule(),
		rb.bufferFullRule(),
		rb.noLogsDeliveredRule(),
	}
}

func (rb fluentBitRuleBuilder) exporterSentRule() Rule {
	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentExporterSentLogs,
		Expr: rate(metricFluentBitOutputProcBytesTotal, selectService(fluentBitMetricsServiceName)).
			sumBy(labelPipelineName).
			greaterThan(0).
			build(),
	}
}

func (rb fluentBitRuleBuilder) exporterDroppedRule() Rule {
	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentExporterDroppedLogs,
		Expr: rate(metricFluentBitOutputDroppedRecordsTotal, selectService(fluentBitMetricsServiceName)).
			sumBy(labelPipelineName).
			greaterThan(0).
			build(),
	}
}

func (rb fluentBitRuleBuilder) bufferInUseRule() Rule {
	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentBufferInUse,
		Expr: instant(metricFluentBitBufferUsageBytes, selectService(fluentBitSidecarMetricsServiceName)).
			greaterThan(bufferUsage300MB).
			build(),
	}
}

func (rb fluentBitRuleBuilder) bufferFullRule() Rule {
	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentBufferFull,
		Expr: instant(metricFluentBitBufferUsageBytes, selectService(fluentBitSidecarMetricsServiceName)).
			greaterThan(bufferUsage900MB).
			build(),
	}
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

	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentNoLogsDelivered,
		Expr:  and(receiverReadExpr, exporterNotSentExpr),
		For:   alertWaitTime,
	}
}

func (rb fluentBitRuleBuilder) namePrefix() string {
	return ruleNamePrefix(typeLogPipeline)
}
