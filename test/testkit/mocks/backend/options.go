package backend

import testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"

type Option func(*Backend)

func WithDropFromSourceLabel(label map[string]string) Option {
	return func(b *Backend) {
		b.dropFromSourceLabel = label
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

func WithMTLS(certs testutils.ServerCerts) Option {
	return func(b *Backend) {
		b.mtls = true
		b.certs = &certs
	}
}

func WithTLS(certs testutils.ServerCerts) Option {
	return func(b *Backend) {
		b.mtls = false
		b.certs = &certs
	}
}

func WithOIDCAuth(issuerURL, audience string) Option {
	return func(b *Backend) {
		b.oidc = &OIDCConfig{
			issuerURL: issuerURL,
			audience:  audience,
		}
	}
}
