package faultbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

const (
	otlpGRPCPortName = "grpc-otlp"
	otlpHTTPPortName = "http-otlp"
	httpLogsPortName = "http-logs"
	configPortName   = "http-config"

	otlpGRPCPort          int32 = 4317
	otlpHTTPPort          int32 = 4318
	httpFluentBitPushPort int32 = 9880
	configPort            int32 = 9090
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

// EnableFaults activates the configured fault rules at runtime by POSTing to the
// mock-backend's /config endpoint on port 9090 via the API server proxy.
// The faultbackend starts healthy (FAULT_RULES="" FAULT_DEFAULT=200); call this
// method after the pipeline reaches FlowHealthy to create a clean rate() baseline
// before faults begin.
func (fb *FaultBackend) EnableFaults(t *testing.T) {
	t.Helper()

	type configRequest struct {
		Rules   *string `json:"rules,omitempty"`
		Default *string `json:"default,omitempty"`
		Delays  *string `json:"delays,omitempty"`
	}

	req := configRequest{}

	rules := fb.faultRulesString()
	req.Rules = &rules

	req.Default = &fb.defaultBehavior

	if len(fb.delays) > 0 {
		delays := fb.faultDelaysString()
		req.Delays = &delays
	}

	body, err := json.Marshal(req)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	proxyURL := suite.ProxyClient.ProxyURLForService(fb.namespace, fb.name, "config", configPort)

	gomega.Eventually(func(g gomega.Gomega) {
		resp, postErr := suite.ProxyClient.Post(context.Background(), proxyURL, "application/json", bytes.NewReader(body))
		g.Expect(postErr).NotTo(gomega.HaveOccurred())
		resp.Body.Close()
		g.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(gomega.Succeed())

	t.Logf("Enabled faults on %s: rules=%s default=%s", proxyURL, rules, fb.defaultBehavior)
}

// buildEnvVars always starts the backend healthy so the pipeline can reach FlowHealthy
// before faults are enabled via EnableFaults().
func (fb *FaultBackend) buildEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "FAULT_RULES", Value: ""},
		{Name: "FAULT_DEFAULT", Value: "200"},
	}
}

func (fb *FaultBackend) buildService() *kitk8sobjects.Service {
	svc := kitk8sobjects.NewService(fb.name, fb.namespace).
		WithPort(otlpGRPCPortName, otlpGRPCPort).
		WithPort(otlpHTTPPortName, otlpHTTPPort).
		WithPort(httpLogsPortName, httpFluentBitPushPort).
		WithPort(configPortName, configPort)

	return svc
}

// faultRulesString returns the fault rules in the "statusCode:percentage,..." format
// expected by the mock-backend /config endpoint.
func (fb *FaultBackend) faultRulesString() string {
	parts := make([]string, 0, len(fb.rules))
	for _, r := range fb.rules {
		parts = append(parts, fmt.Sprintf("%d:%.0f", r.statusCode, r.percentage))
	}

	return strings.Join(parts, ",")
}

// faultDelaysString returns the fault delays in the "statusCode:delayMs,..." format.
func (fb *FaultBackend) faultDelaysString() string {
	parts := make([]string, 0, len(fb.delays))
	for code, d := range fb.delays {
		parts = append(parts, fmt.Sprintf("%d:%d", code, d.Milliseconds()))
	}

	return strings.Join(parts, ",")
}
