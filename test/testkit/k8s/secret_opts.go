package k8s

import (
	"github.com/kyma-project/telemetry-manager/test/testkit"
)

type secretOptions struct {
	stringData map[string]string
}

func WithStringData(key, value string) testkit.OptFunc {
	return func(options testkit.Opt) {
		switch opt := options.(type) {
		case secretOptions:
			opt.stringData[key] = value
		}
	}
}

func processSecretOptions(opts ...testkit.OptFunc) secretOptions {
	options := secretOptions{stringData: make(map[string]string)}

	for _, opt := range opts {
		opt(options)
	}

	return options
}
