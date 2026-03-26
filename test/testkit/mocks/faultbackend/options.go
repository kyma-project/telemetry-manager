package faultbackend

import (
	"fmt"
	"time"
)

type Option func(*FaultBackend)

// WithStatusCodeAndPercentage adds a fault rule: the given percentage of requests will receive
// the specified HTTP status code. Can be called multiple times for different codes.
// The total percentage across all rules must not exceed 100.
func WithStatusCodeAndPercentage(statusCode int32, percentage float64) Option {
	return func(fb *FaultBackend) {
		fb.rules = append(fb.rules, rule{statusCode: statusCode, percentage: percentage})
	}
}

func WithDefaultStatusCode(statusCode int32) Option {
	return func(fb *FaultBackend) {
		fb.defaultBehavior = fmt.Sprintf("%d", statusCode)
	}
}

func WithDefaultClose() Option {
	return func(fb *FaultBackend) {
		fb.defaultBehavior = "close"
	}
}

func WithName(name string) Option {
	return func(fb *FaultBackend) {
		fb.name = name
	}
}

func WithReplicas(replicas int32) Option {
	return func(fb *FaultBackend) {
		fb.replicas = replicas
	}
}

// WithFluentBitPort makes Port() return the Fluent Bit HTTP push port (9880) instead
// of the OTLP gRPC port (4317). Use this when the FaultBackend targets a FluentBit LogPipeline.
func WithFluentBitPort() Option {
	return func(fb *FaultBackend) {
		fb.useFluentBitPort = true
	}
}

// WithDelay configures a response delay for a specific HTTP status code.
// The fault-backend will sleep for the given duration before sending the response.
// The body is drained before sleeping, so only the goroutine stack is held during the delay.
func WithDelay(statusCode int32, delay time.Duration) Option {
	return func(fb *FaultBackend) {
		if fb.delays == nil {
			fb.delays = make(map[int32]time.Duration)
		}

		fb.delays[statusCode] = delay
	}
}
