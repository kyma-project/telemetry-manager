package selfmonitor

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
)

// FaultEnabler is implemented by both FaultBackend (runtime /config POST) and
// istioFaultEnabler (VirtualService applied at runtime). Tests call EnableFaults
// after the pipeline reaches FlowHealthy to get a clean rate() baseline first.
type FaultEnabler interface {
	EnableFaults(t *testing.T)
}

// istioFaultEnabler applies a pre-built VirtualService at runtime via kitk8s.CreateObjects.
// Use this for metric-agent tests where the VS must selectively block only
// agent→backend traffic without affecting the gateway's own backend traffic.
type istioFaultEnabler struct {
	vs *kitk8sobjects.VirtualService
}

// newIstioFaultEnabler creates an istioFaultEnabler that injects a gRPC INVALID_ARGUMENT fault.
// A gRPC status fault is used because the metric agent communicates with the backend over gRPC
// (port 4317). Using an HTTP abort on gRPC traffic produces a transport-level error that the
// OTel gRPC exporter does not count as send_failed, so the self-monitor alert never fires.
// INVALID_ARGUMENT is a non-retryable gRPC status code, causing the exporter to drop data
// immediately and increment otelcol_exporter_send_failed_metric_points_total.
func newIstioFaultEnabler(name, namespace, host string, percentage float64, sourceLabel map[string]string) *istioFaultEnabler {
	vs := kitk8sobjects.NewVirtualService(name, namespace, host).
		WithFaultAbortGrpcStatus(percentage, "INVALID_ARGUMENT").
		WithSourceLabel(sourceLabel)

	return &istioFaultEnabler{vs: vs}
}

func (e *istioFaultEnabler) EnableFaults(t *testing.T) {
	t.Helper()
	t.Log("Enabling faults via Istio VirtualService")
	Expect(kitk8s.CreateObjects(t, e.vs.K8sObject())).To(Succeed())
}

// K8sObjects returns the VirtualService as a Kubernetes object slice so it can be
// included in the initial resource set when no healthy baseline is required.
func (e *istioFaultEnabler) K8sObjects() []client.Object {
	return []client.Object{e.vs.K8sObject()}
}
