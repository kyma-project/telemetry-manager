package k8s

import (
	"github.com/kyma-project/telemetry-manager/test/testkit"
)

type Labels map[string]string

const (
	VersionLabelName    = "version"
	PersistentLabelName = "persistent"
)

var (
	PersistentLabel = Labels{PersistentLabelName: "true"}
)

// Version sets a property corresponding to a version to the passed value.
func (l *Labels) Version(v string) {
	if *l == nil {
		*l = make(Labels, 0)
	}
	(*l)[VersionLabelName] = v
}

// WithLabel is a functional option for attaching a label value.
func WithLabel(label, value string) testkit.OptFunc {
	return func(opt testkit.Opt) {
		switch x := opt.(type) {
		case Labels:
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
