package backend

import (
	"github.com/kyma-project/telemetry-manager/test/testkit"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
)

type OptionSetter func(*Options)

type Options struct {
	Name                     string
	SignalType               SignalType
	WithPersistentHostSecret bool

	WithTLS  bool
	TLSCerts testkit.TLSCerts

	TracePipelineOptions  []kittrace.PipelineOption
	MetricPipelineOptions []kitmetric.PipelineOption
}

func NewOptions(name string, setters ...OptionSetter) *Options {
	options := &Options{
		Name: name,
	}
	for _, setter := range setters {
		setter(options)
	}
	return options
}

func WithTLS() OptionSetter {
	return func(o *Options) {
		o.WithTLS = true
	}
}

func WithMetricPipelineOption(option kitmetric.PipelineOption) OptionSetter {
	return func(o *Options) {
		o.MetricPipelineOptions = append(o.MetricPipelineOptions, option)
	}
}
