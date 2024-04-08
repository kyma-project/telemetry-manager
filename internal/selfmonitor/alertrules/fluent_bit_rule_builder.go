package alertrules

const (
	fluentBitMetricsServiceName = "telemetry-fluent-bit-metrics\""
)

type fluentBitRuleBuilder struct {
	namePrefix string
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
		Alert: RuleNameLogAgentExporterSentLogs,
		Expr: rate("fluentbit_output_bytes_total", selectService(fluentBitMetricsServiceName)).
			greaterThan(0).
			build(),
	}
}

func (rb fluentBitRuleBuilder) receiverReadRule() Rule {
	return Rule{
		Alert: RuleNameLogAgentReceiverReadLogs,
		Expr: rate("fluentbit_input_bytes_total", selectService(fluentBitMetricsServiceName)).
			greaterThan(0).
			build(),
	}
}

func (rb fluentBitRuleBuilder) exporterDroppedRule() Rule {
	return Rule{
		Alert: RuleNameLogAgentExporterDroppedLogs,
		Expr: rate("fluentbit_output_retries_failed_total", selectService(fluentBitMetricsServiceName)).
			greaterThan(0).
			build(),
	}
}

func (rb fluentBitRuleBuilder) bufferInUseRule() Rule {
	return Rule{
		Alert: RuleNameLogAgentBufferInUse,
		Expr:  "telemetry_fsbuffer_usage_bytes > 300000000",
	}
}

func (rb fluentBitRuleBuilder) bufferFullRule() Rule {
	return Rule{
		Alert: RuleNameLogAgentBufferFull,
		Expr:  "telemetry_fsbuffer_usage_bytes > 900000000",
	}
}
