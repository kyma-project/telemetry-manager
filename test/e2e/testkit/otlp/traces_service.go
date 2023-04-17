//go:build e2e

package otlp

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kyma-project/telemetry-manager/test/e2e/testkit"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
)

type TracesService struct {
	name      string
	namespace string
	testkit.PortRegistry
}

func NewTracesService(name, namespace string) *TracesService {
	return &TracesService{
		name:         name,
		namespace:    namespace,
		PortRegistry: testkit.NewPortRegistry(),
	}
}

func (s *TracesService) WithPortMapping(name string, port, nodePort int32) *TracesService {
	s.PortRegistry.AddPortMapping(name, port, nodePort)
	return s
}

func (s *TracesService) K8sObject(labelOpts ...testkit.OptFunc) *corev1.Service {
	labels := k8s.ProcessLabelOptions(labelOpts...)

	ports := make([]corev1.ServicePort, 0)
	for name, mapping := range s.PortRegistry.Ports {
		ports = append(ports, corev1.ServicePort{
			Name:       name,
			Protocol:   corev1.ProtocolTCP,
			Port:       mapping.Port,
			TargetPort: intstr.FromInt(int(mapping.Port)),
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
