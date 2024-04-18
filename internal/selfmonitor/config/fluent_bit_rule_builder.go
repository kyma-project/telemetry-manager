package config

import (
	"fmt"
)

const (
	fluentBitMetricsServiceName = "telemetry-fluent-bit-metrics"

	metricFluentBitOutputProcBytesTotal = "fluentbit_output_proc_bytes_total"
	metricFluentBitInputBytesTotal      = "fluentbit_input_bytes_total"
	metricFluentBitOutputDroppedRecords = "fluentbit_output_dropped_records_total"
	metricFluentBitBufferUsageBytes     = "telemetry_fsbuffer_usage_bytes"
)

type fluentBitRuleBuilder struct {
}

func (rb fluentBitRuleBuilder) rules() []Rule {
	return []Rule{
		rb.exporterSentRule(),
		rb.receiverReadRule(),
		rb.exporterDroppedRule(),
		rb.bufferInUseRule(),
		rb.bufferFullRule(),
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

func (rb fluentBitRuleBuilder) receiverReadRule() Rule {
	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentReceiverReadLogs,
		Expr: rate(metricFluentBitInputBytesTotal, selectService(fluentBitMetricsServiceName)).
			sumBy(labelPipelineName).
			greaterThan(0).
			build(),
	}
}

func (rb fluentBitRuleBuilder) exporterDroppedRule() Rule {
	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentExporterDroppedLogs,
		Expr: rate(metricFluentBitOutputDroppedRecords, selectService(fluentBitMetricsServiceName)).
			sumBy(labelPipelineName).
			greaterThan(0).
			build(),
	}
}

func (rb fluentBitRuleBuilder) bufferInUseRule() Rule {
	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentBufferInUse,
		Expr:  fmt.Sprintf("%s > 300000000", metricFluentBitBufferUsageBytes),
	}
}

func (rb fluentBitRuleBuilder) bufferFullRule() Rule {
	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentBufferFull,
		Expr:  fmt.Sprintf("%s > 900000000", metricFluentBitBufferUsageBytes),
	}
}

func (rb fluentBitRuleBuilder) namePrefix() string {
	return ruleNamePrefix(typeLogPipeline)
}
