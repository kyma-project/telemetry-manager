package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kyma-project/telemetry-manager/test/testkit"
)

type Service struct {
	testkit.PortRegistry

	name      string
	namespace string
}

func NewService(name, namespace string) *Service {
	return &Service{
		name:         name,
		namespace:    namespace,
		PortRegistry: testkit.NewPortRegistry(),
	}
}

func (s *Service) WithPort(name string, port int) *Service {
	s.PortRegistry.AddPort(name, port)
	return s
}

func (s *Service) K8sObject(labelOpts ...testkit.OptFunc) *corev1.Service {
	labels := ProcessLabelOptions(labelOpts...)

	ports := make([]corev1.ServicePort, 0)
	for name, port := range s.PortRegistry.Ports {
		ports = append(ports, corev1.ServicePort{
			Name:       name,
			Protocol:   corev1.ProtocolTCP,
			Port:       port,
			TargetPort: intstr.FromInt(int(port)),
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
