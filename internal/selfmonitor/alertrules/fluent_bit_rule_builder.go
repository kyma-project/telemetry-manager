package alertrules

const (
	fluentBitMetricsServiceName = "telemetry-fluent-bit-metrics"
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
		Expr: rate("fluentbit_output_proc_bytes_total", selectService(fluentBitMetricsServiceName)).
			sumBy(labelPipelineName).
			greaterThan(0).
			build(),
	}
}

func (rb fluentBitRuleBuilder) receiverReadRule() Rule {
	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentReceiverReadLogs,
		Expr: rate("fluentbit_input_bytes_total", selectService(fluentBitMetricsServiceName)).
			sumBy(labelPipelineName).
			greaterThan(0).
			build(),
	}
}

func (rb fluentBitRuleBuilder) exporterDroppedRule() Rule {
	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentExporterDroppedLogs,
		Expr: rate("fluentbit_output_dropped_records_total", selectService(fluentBitMetricsServiceName)).
			sumBy(labelPipelineName).
			greaterThan(0).
			build(),
	}
}

func (rb fluentBitRuleBuilder) bufferInUseRule() Rule {
	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentBufferInUse,
		Expr:  "telemetry_fsbuffer_usage_bytes > 300000000",
	}
}

func (rb fluentBitRuleBuilder) bufferFullRule() Rule {
	return Rule{
		Alert: rb.namePrefix() + RuleNameLogAgentBufferFull,
		Expr:  "telemetry_fsbuffer_usage_bytes > 900000000",
	}
}

func (rb fluentBitRuleBuilder) namePrefix() string {
	return ruleNamePrefix(typeLogPipeline)
}
