package faultbackend

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
)

const (
	otlpGRPCPortName = "grpc-otlp"
	otlpHTTPPortName = "http-otlp"
	httpLogsPortName = "http-logs"

	otlpGRPCPort          int32 = 4317
	otlpHTTPPort          int32 = 4318
	httpFluentBitPushPort int32 = 9880
)

const DefaultName = "fault-backend"

type rule struct {
	statusCode int32
	percentage float64
}

// FaultBackend deploys a lightweight mock server that returns configurable HTTP status codes
// at configurable percentages. It replaces Istio VirtualService-based fault injection for
// self-monitor tests.
type FaultBackend struct {
	name             string
	namespace        string
	replicas         int32
	rules            []rule
	defaultBehavior  string
	useFluentBitPort bool
	delays           map[int32]time.Duration
}

func New(namespace string, opts ...Option) *FaultBackend {
	fb := &FaultBackend{
		name:            DefaultName,
		namespace:       namespace,
		replicas:        1,
		defaultBehavior: "200",
	}

	for _, opt := range opts {
		opt(fb)
	}

	return fb
}

func (fb *FaultBackend) Name() string {
	return fb.name
}

func (fb *FaultBackend) Namespace() string {
	return fb.namespace
}

func (fb *FaultBackend) Host() string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", fb.name, fb.namespace)
}

func (fb *FaultBackend) Port() int32 {
	if fb.useFluentBitPort {
		return httpFluentBitPushPort
	}

	return otlpGRPCPort
}

func (fb *FaultBackend) EndpointHTTP() string {
	addr := net.JoinHostPort(fb.Host(), strconv.Itoa(int(fb.Port())))
	return fmt.Sprintf("http://%s", addr)
}

func (fb *FaultBackend) K8sObjects() []client.Object {
	labels := kitk8sobjects.WithLabel("app", fb.name)

	return []client.Object{
		fb.buildDeployment(),
		fb.buildService().K8sObject(labels),
	}
}

func (fb *FaultBackend) buildEnvVars() []corev1.EnvVar {
	var envs []corev1.EnvVar

	if len(fb.rules) > 0 {
		parts := make([]string, 0, len(fb.rules))
		for _, r := range fb.rules {
			parts = append(parts, fmt.Sprintf("%d:%.0f", r.statusCode, r.percentage))
		}

		envs = append(envs, corev1.EnvVar{Name: "FAULT_RULES", Value: strings.Join(parts, ",")})
	}

	envs = append(envs, corev1.EnvVar{Name: "FAULT_DEFAULT", Value: fb.defaultBehavior})

	if len(fb.delays) > 0 {
		parts := make([]string, 0, len(fb.delays))
		for code, d := range fb.delays {
			parts = append(parts, fmt.Sprintf("%d:%d", code, d.Milliseconds()))
		}

		envs = append(envs, corev1.EnvVar{Name: "FAULT_DELAYS", Value: strings.Join(parts, ",")})
	}

	return envs
}

func (fb *FaultBackend) buildService() *kitk8sobjects.Service {
	svc := kitk8sobjects.NewService(fb.name, fb.namespace).
		WithPort(otlpGRPCPortName, otlpGRPCPort).
		WithPort(otlpHTTPPortName, otlpHTTPPort).
		WithPort(httpLogsPortName, httpFluentBitPushPort)

	return svc
}
