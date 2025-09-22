package backend

import testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"

type Option func(*Backend)

func WithSourceMetricAgent() Option {
	return func(b *Backend) {
		b.sourceMetricAgent = true
	}
}

func WithAbortFaultInjection(abortFaultPercentage float64) Option {
	return func(b *Backend) {
		b.abortFaultPercentage = abortFaultPercentage
	}
}

func WithName(name string) Option {
	return func(b *Backend) {
		b.name = name
	}
}

func WithReplicas(replicas int32) Option {
	return func(b *Backend) {
		b.replicas = replicas
	}
}

func WithTLS(certKey testutils.ServerCerts) Option {
	return func(b *Backend) {
		b.certs = &certKey
	}
}
