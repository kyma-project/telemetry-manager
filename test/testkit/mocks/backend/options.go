package backend

import (
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
)

type Options struct {
	Namespace             string
	WithTLS               bool
	MockDeploymentNames   []string
	TracePipelineOptions  []kittrace.PipelineOption
	MetricPipelineOptions []kitmetric.PipelineOption
}

type OptionSetter func(*Options)

func WithTLS(withTLS bool) OptionSetter {
	return func(o *Options) {
		o.WithTLS = withTLS
	}
}

func WithMockNamespace(namespace string) OptionSetter {
	return func(o *Options) {
		o.Namespace = namespace
	}
}

func WithMockDeploymentNames(names ...string) OptionSetter {
	return func(o *Options) {
		o.MockDeploymentNames = names
	}
}

func WithTracePipelineOption(option kittrace.PipelineOption) OptionSetter {
	return func(o *Options) {
		o.TracePipelineOptions = append(o.TracePipelineOptions, option)
	}
}

func WithMetricPipelineOption(option kitmetric.PipelineOption) OptionSetter {
	return func(o *Options) {
		o.MetricPipelineOptions = append(o.MetricPipelineOptions, option)
	}
}
