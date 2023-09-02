package mocks

import (
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
)

type BackendOptions struct {
	Namespace             string
	WithTLS               bool
	MockDeploymentNames   []string
	TracePipelineOptions  []kittrace.PipelineOption
	MetricPipelineOptions []kitmetric.PipelineOption
}

type BackendOptionSetter func(*BackendOptions)

func WithTLS(withTLS bool) BackendOptionSetter {
	return func(o *BackendOptions) {
		o.WithTLS = withTLS
	}
}

func WithMockNamespace(namespace string) BackendOptionSetter {
	return func(o *BackendOptions) {
		o.Namespace = namespace
	}
}

func WithMockDeploymentNames(names ...string) BackendOptionSetter {
	return func(o *BackendOptions) {
		o.MockDeploymentNames = names
	}
}

func WithTracePipelineOption(option kittrace.PipelineOption) BackendOptionSetter {
	return func(o *BackendOptions) {
		o.TracePipelineOptions = append(o.TracePipelineOptions, option)
	}
}

func WithMetricPipelineOption(option kitmetric.PipelineOption) BackendOptionSetter {
	return func(o *BackendOptions) {
		o.MetricPipelineOptions = append(o.MetricPipelineOptions, option)
	}
}
