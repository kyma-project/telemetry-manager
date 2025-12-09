package objects

import (
	"github.com/kyma-project/telemetry-manager/test/testkit"
)

type Labels map[string]string

const (
	PersistentLabelName = "persistent"
)

var (
	PersistentLabel = Labels{PersistentLabelName: "true"}
)

// WithLabel is a functional option for attaching a label value.
func WithLabel(label, value string) testkit.OptFunc {
	return func(opt testkit.Opt) {
		if x, ok := opt.(Labels); ok {
			x[label] = value
		}
	}
}

// ProcessLabelOptions returns the map of labels attached using WithLabel.
func ProcessLabelOptions(opts ...testkit.OptFunc) Labels {
	labels := make(Labels)

	for _, opt := range opts {
		opt(labels)
	}

	return labels
}
