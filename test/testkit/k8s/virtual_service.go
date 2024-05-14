package k8s

import (
	"net/http"

	istionetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VirtualService struct {
	name                 string
	namespace            string
	host                 string
	abortFaultPercentage float64
}

func NewVirtualService(name, namespace, host string) *VirtualService {
	return &VirtualService{
		name:      name,
		namespace: namespace,
		host:      host,
	}
}

func (s *VirtualService) WithAbortFaultPercentage(percentage float64) *VirtualService {
	s.abortFaultPercentage = percentage
	return s
}

func (s *VirtualService) K8sObject() *istionetworkingclientv1beta1.VirtualService {
	return &istionetworkingclientv1beta1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
		},
		Spec: istionetworkingv1beta1.VirtualService{
			Hosts: []string{s.host},
			Http: []*istionetworkingv1beta1.HTTPRoute{
				{
					Route: []*istionetworkingv1beta1.HTTPRouteDestination{
						{
							Destination: &istionetworkingv1beta1.Destination{
								Host: s.host,
							},
						},
					},
					Fault: &istionetworkingv1beta1.HTTPFaultInjection{
						Abort: &istionetworkingv1beta1.HTTPFaultInjection_Abort{
							Percentage: &istionetworkingv1beta1.Percent{
								Value: s.abortFaultPercentage,
							},
							ErrorType: &istionetworkingv1beta1.HTTPFaultInjection_Abort_HttpStatus{
								HttpStatus: http.StatusBadGateway,
							},
						},
					},
				},
			},
		},
	}
}
