package selfmonitor

import (
	"net/http"
	"testing"

	. "github.com/onsi/gomega"

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
// agent→gateway traffic without affecting the gateway's own backend traffic.
type istioFaultEnabler struct {
	vs *kitk8sobjects.VirtualService
}

func newIstioFaultEnabler(name, namespace, host string, percentage float64, sourceLabel map[string]string) *istioFaultEnabler {
	vs := kitk8sobjects.NewVirtualService(name, namespace, host).
		WithFaultAbortPercentage(percentage, http.StatusBadRequest).
		WithSourceLabel(sourceLabel)

	return &istioFaultEnabler{vs: vs}
}

func (e *istioFaultEnabler) EnableFaults(t *testing.T) {
	t.Helper()
	Expect(kitk8s.CreateObjects(t, e.vs.K8sObject())).To(Succeed())
}
