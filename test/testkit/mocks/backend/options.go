package backend

import testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"

type Option func(*Backend)

func WithAbortFaultInjection(abortFaultPercentage float64) Option {
	return func(b *Backend) {
		b.abortFaultPercentage = abortFaultPercentage
	}
}

func WithFaultDelayInjection(faultPercentage float64, delaySeconds int) Option {
	return func(b *Backend) {
		b.faultDelayPercentage = faultPercentage
		b.faultDelayFixedDelaySeconds = delaySeconds
	}
}

func WithName(name string) Option {
	return func(b *Backend) {
		b.name = name
	}
}

func WithPersistentHostSecret(persistentHostSecret bool) Option {
	return func(b *Backend) {
		b.persistentHostSecret = persistentHostSecret
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
