package backend

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s"
)

type ExternalService struct {
	testkit.PortRegistry

	name      string
	namespace string
}

func NewExternalService(name, namespace string) *ExternalService {
	return &ExternalService{
		name:         name,
		namespace:    namespace,
		PortRegistry: testkit.NewPortRegistry(),
	}
}

func (s *ExternalService) OTLPEndpointURL(port int) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", s.name, s.namespace, port)
}

func (s *ExternalService) Host() string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", s.name, s.namespace)
}

func (s *ExternalService) WithPort(name string, port int) *ExternalService {
	s.PortRegistry.AddPort(name, port)
	return s
}

func (s *ExternalService) K8sObject(labelOpts ...testkit.OptFunc) *corev1.Service {
	labels := k8s.ProcessLabelOptions(labelOpts...)

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
