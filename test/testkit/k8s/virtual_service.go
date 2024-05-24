package k8s

import (
	"net/http"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	istionetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VirtualService struct {
	name                 string
	namespace            string
	host                 string
	faultAbortPercentage float64
	faultDelayPercentage float64
	faultDelayFixedDelay time.Duration
}

type Option func(*VirtualService)

func NewVirtualService(name, namespace, host string) *VirtualService {
	return &VirtualService{
		name:      name,
		namespace: namespace,
		host:      host,
	}
}

func (s *VirtualService) WithFaultAbortPercentage(percentage float64) *VirtualService {
	s.faultAbortPercentage = percentage
	return s
}

func (s *VirtualService) WithFaultDelay(percentage float64, delay time.Duration) *VirtualService {
	s.faultDelayPercentage = percentage
	s.faultDelayFixedDelay = delay
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
						Delay: func() *istionetworkingv1beta1.HTTPFaultInjection_Delay {
							if s.faultDelayPercentage == 0 {
								return nil
							}
							return &istionetworkingv1beta1.HTTPFaultInjection_Delay{
								HttpDelayType: &istionetworkingv1beta1.HTTPFaultInjection_Delay_FixedDelay{FixedDelay: durationpb.New(s.faultDelayFixedDelay)},
								Percentage: &istionetworkingv1beta1.Percent{
									Value: s.faultDelayPercentage,
								},
							}
						}(),
						Abort: func() *istionetworkingv1beta1.HTTPFaultInjection_Abort {
							if s.faultAbortPercentage == 0 {
								return nil
							}
							return &istionetworkingv1beta1.HTTPFaultInjection_Abort{Percentage: &istionetworkingv1beta1.Percent{
								Value: s.faultAbortPercentage,
							},
								ErrorType: &istionetworkingv1beta1.HTTPFaultInjection_Abort_HttpStatus{
									HttpStatus: http.StatusBadGateway,
								},
							}
						}(),
					},
				},
			},
		},
	}
}
