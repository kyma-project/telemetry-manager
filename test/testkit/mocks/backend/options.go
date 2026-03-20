package backend

import (
	"time"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

type Option func(*Backend)

func WithDropFromSourceLabel(label map[string]string) Option {
	return func(b *Backend) {
		b.dropFromSourceLabel = label
	}
}

// WithAbortFaultInjection configures an Istio VirtualService HTTP abort for a fraction of requests.
// Use 400 (Bad Request) for non-retryable and 429 (Too Many Requests) for retryable behavior consistent
// with OTel Collector and Fluent Bit (see test/selfmonitor/helpers.go).
func WithAbortFaultInjection(abortFaultPercentage float64, statusCode int32) Option {
	return func(b *Backend) {
		b.abortFaultPercentage = abortFaultPercentage
		b.abortFaultStatusCode = statusCode
	}
}

func WithDelayFaultInjection(percentage float64, delay time.Duration) Option {
	return func(b *Backend) {
		b.delayFaultPercentage = percentage
		b.delayFaultDuration = delay
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
