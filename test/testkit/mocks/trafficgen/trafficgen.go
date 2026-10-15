package trafficgen

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
)

const (
	nginxImage      = "europe-docker.pkg.dev/kyma-project/prod/external/nginx:1.23.3"
	curlImage       = "europe-docker.pkg.dev/kyma-project/prod/external/curlimages/curl:7.78.0"
	sourceName      = "source"
	destinationName = "destination"
	destinationPort = int32(80)
	appLabelKey     = "app"
)

type TrafficGen struct {
	namespace          string
	traceparentHeaders []string
}

func New(namespace string) *TrafficGen {
	return &TrafficGen{namespace: namespace}
}

// WithTraceparentHeaders configures the source pod to send one request per traceparent
// value (in order) and then stop. Use this when you need to verify specific trace IDs
// in the backend. Without this option the source pod loops indefinitely.
func (t *TrafficGen) WithTraceparentHeaders(traceparents []string) *TrafficGen {
	t.traceparentHeaders = traceparents
	return t
}

// Destination returns a view that exposes only the destination (nginx + service) objects.
// Deploy these first and wait for readiness before deploying the source.
func (t *TrafficGen) Destination() *Destination {
	return &Destination{tg: t}
}

// Source returns a view that exposes only the source (curl) object.
// Deploy after the destination is ready.
func (t *TrafficGen) Source() *Source {
	return &Source{tg: t}
}

// K8sObjects returns all objects (source + destination) in one call.
// Use this for continuous-traffic scenarios where ordering doesn't matter.
func (t *TrafficGen) K8sObjects() []client.Object {
	return append(t.Destination().K8sObjects(), t.Source().K8sObjects()...)
}

// K8sObjects is a convenience function for the common case of continuous traffic with no custom headers.
func K8sObjects(namespace string) []client.Object {
	return New(namespace).K8sObjects()
}

type Destination struct{ tg *TrafficGen }

func (d *Destination) K8sObjects() []client.Object {
	return []client.Object{
		kitk8sobjects.NewPod(destinationName, d.tg.namespace).WithPodSpec(destinationPodSpec()).WithLabel(appLabelKey, destinationName).K8sObject(),
		kitk8sobjects.NewService(destinationName, d.tg.namespace).WithPort("http", destinationPort).K8sObject(kitk8sobjects.WithLabel(appLabelKey, destinationName)),
	}
}

type Source struct{ tg *TrafficGen }

func (s *Source) K8sObjects() []client.Object {
	return []client.Object{
		kitk8sobjects.NewPod(sourceName, s.tg.namespace).WithPodSpec(s.tg.sourcePodSpec()).K8sObject(),
	}
}

func (t *TrafficGen) sourcePodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:    "source",
				Image:   curlImage,
				Command: []string{"/bin/sh", "-c", t.curlScript()},
			},
		},
	}
}

func (t *TrafficGen) curlScript() string {
	if len(t.traceparentHeaders) == 0 {
		return fmt.Sprintf("while true; do curl http://%s:%d; sleep 1; done", destinationName, destinationPort)
	}

	var sb strings.Builder

	// Wait for Envoy sidecar to be ready before sending traced requests.
	// Without this, curl fires before the sidecar proxy is up and the spans are lost.
	sb.WriteString("until curl -sf http://localhost:15000/ready; do sleep 1; done; ")

	for _, tp := range t.traceparentHeaders {
		fmt.Fprintf(&sb, "curl -H 'traceparent: %s' http://%s:%d; ", tp, destinationName, destinationPort)
	}

	return sb.String()
}

func destinationPodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "destination",
				Image: nginxImage,
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: destinationPort,
						Protocol:      corev1.ProtocolTCP,
					},
				},
			},
		},
	}
}
