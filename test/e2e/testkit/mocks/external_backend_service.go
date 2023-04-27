//go:build e2e

package mocks

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kyma-project/telemetry-manager/test/e2e/testkit"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
)

type ExternalBackendService struct {
	testkit.PortRegistry

	name      string
	namespace string
}

func NewExternalBackendService(name, namespace string) *ExternalBackendService {
	return &ExternalBackendService{
		name:         name,
		namespace:    namespace,
		PortRegistry: testkit.NewPortRegistry(),
	}
}

func (s *ExternalBackendService) OTLPEndpointURL(port int32) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", s.name, s.namespace, port)
}

func (s *ExternalBackendService) WithPort(name string, port int32) *ExternalBackendService {
	s.PortRegistry.AddServicePort(name, port)
	return s
}

func (s *ExternalBackendService) WithPortMapping(name string, port, nodePort int32) *ExternalBackendService {
	s.PortRegistry.AddPortMapping(name, port, nodePort, 0)
	return s
}

func (s *ExternalBackendService) K8sObject(labelOpts ...testkit.OptFunc) *corev1.Service {
	labels := k8s.ProcessLabelOptions(labelOpts...)

	ports := make([]corev1.ServicePort, 0)
	for name, mapping := range s.PortRegistry.Ports {
		ports = append(ports, corev1.ServicePort{
			Name:       name,
			Protocol:   corev1.ProtocolTCP,
			Port:       mapping.ServicePort,
			TargetPort: intstr.FromInt(int(mapping.ServicePort)),
			NodePort:   mapping.NodePort,
		})
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports:    ports,
			Selector: labels,
			Type:     corev1.ServiceTypeNodePort,
		},
	}
}
