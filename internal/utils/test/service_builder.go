package test

import (
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// ServiceBuilder is a test utility for building corev1.Service objects.
type ServiceBuilder struct {
	service corev1.Service
}

func NewServiceBuilder() *ServiceBuilder {
	return &ServiceBuilder{
		service: corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "kyma-system",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "test"},
				Ports: []corev1.ServicePort{
					{Port: 80, Protocol: corev1.ProtocolTCP},
				},
			},
		},
	}
}

func (b *ServiceBuilder) WithName(name string) *ServiceBuilder {
	b.service.Name = name
	return b
}

func (b *ServiceBuilder) WithNamespace(namespace string) *ServiceBuilder {
	b.service.Namespace = namespace
	return b
}

func (b *ServiceBuilder) Build() corev1.Service {
	return b.service
}

// EndpointBuilder is a test utility for building corev1.Endpoints objects.
type EndpointSliceBuilder struct {
	endpointSlice discoveryv1.EndpointSlice
}

func NewEndpointSliceBuilder() *EndpointSliceBuilder {
	return &EndpointSliceBuilder{
		endpointSlice: discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "kyma-system",
				Labels: map[string]string{
					"app": "test",
				},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"127.0.0.1"},
					Conditions: discoveryv1.EndpointConditions{
						Ready: ptr.To(true),
					},
				},
			},
			Ports: []discoveryv1.EndpointPort{},
		},
	}
}

func (b *EndpointSliceBuilder) Build() discoveryv1.EndpointSlice {
	return b.endpointSlice
}

func (b *EndpointSliceBuilder) WithName(name string) *EndpointSliceBuilder {
	b.endpointSlice.ObjectMeta.Name = name
	return b
}

func (b *EndpointSliceBuilder) WithNamespace(namespace string) *EndpointSliceBuilder {
	b.endpointSlice.ObjectMeta.Namespace = namespace
	return b
}

func (b *EndpointSliceBuilder) WithLabel(label map[string]string) *EndpointSliceBuilder {
	b.endpointSlice.ObjectMeta.Labels = label
	return b
}
