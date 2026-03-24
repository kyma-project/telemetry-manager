package objects

import (
	"net/http"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	istionetworkingv1 "istio.io/api/networking/v1"
	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VirtualService builds an Istio VirtualService with optional HTTP fault injection (delay / abort).
//
// For self-monitor and mock-backend tests, align HTTP abort status with exporter retry semantics:
//   - 400 Bad Request: non-retryable for both OTel Collector and Fluent Bit (see test/selfmonitor/helpers.go).
//   - 429 Too Many Requests: retryable for both (used e.g. in faultRetryableErr in test/selfmonitor/helpers.go).
//
// Call WithFaultAbortPercentage(percentage, httpStatus) so the abort uses the intended code; the default
// in NewVirtualService matches the non-retryable convention (400) if a caller ever omits the status.
//
// sourceLabels is an Istio selector (NOT a runtime match): it determines which sidecar proxies
// receive this VirtualService config. Pods whose labels don't match never see the VS at all,
// so no fallback route is needed for non-matching workloads.
type VirtualService struct {
	name                 string
	namespace            string
	host                 string
	faultAbortPercentage float64
	faultDelayPercentage float64
	sourceLabel          map[string]string
	faultDelayFixedDelay time.Duration
	faultAbortHttpStatus int32
}

type Option func(*VirtualService)

func NewVirtualService(name, namespace, host string) *VirtualService {
	return &VirtualService{
		name:                 name,
		namespace:            namespace,
		host:                 host,
		faultAbortHttpStatus: http.StatusBadRequest,
	}
}

func (s *VirtualService) WithSourceLabel(sourceLabel map[string]string) *VirtualService {
	s.sourceLabel = sourceLabel
	return s
}

// WithFaultAbortPercentage sets the fraction of requests that receive an HTTP fault abort with httpStatus
// (e.g. http.StatusBadRequest for non-retryable, http.StatusTooManyRequests for retryable).
func (s *VirtualService) WithFaultAbortPercentage(percentage float64, httpStatus int32) *VirtualService {
	s.faultAbortPercentage = percentage
	s.faultAbortHttpStatus = httpStatus

	return s
}

func (s *VirtualService) WithFaultDelay(percentage float64, delay time.Duration) *VirtualService {
	s.faultDelayPercentage = percentage
	s.faultDelayFixedDelay = delay

	return s
}

func (s *VirtualService) K8sObject() *istionetworkingclientv1.VirtualService {
	return &istionetworkingclientv1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
		},
		Spec: istionetworkingv1.VirtualService{
			Hosts: []string{s.host},
			Http: []*istionetworkingv1.HTTPRoute{
				{
					Match: []*istionetworkingv1.HTTPMatchRequest{
						{
							SourceLabels: s.sourceLabel,
						},
					},
					Route: []*istionetworkingv1.HTTPRouteDestination{
						{
							Destination: &istionetworkingv1.Destination{
								Host: s.host,
							},
						},
					},
					Fault: &istionetworkingv1.HTTPFaultInjection{
						Delay: func() *istionetworkingv1.HTTPFaultInjection_Delay {
							if s.faultDelayPercentage == 0 {
								return nil
							}

							return &istionetworkingv1.HTTPFaultInjection_Delay{
								HttpDelayType: &istionetworkingv1.HTTPFaultInjection_Delay_FixedDelay{FixedDelay: durationpb.New(s.faultDelayFixedDelay)},
								Percentage: &istionetworkingv1.Percent{
									Value: s.faultDelayPercentage,
								},
							}
						}(),
						Abort: func() *istionetworkingv1.HTTPFaultInjection_Abort {
							if s.faultAbortPercentage == 0 {
								return nil
							}

							return &istionetworkingv1.HTTPFaultInjection_Abort{Percentage: &istionetworkingv1.Percent{
								Value: s.faultAbortPercentage,
							},
								ErrorType: &istionetworkingv1.HTTPFaultInjection_Abort_HttpStatus{
									HttpStatus: s.faultAbortHttpStatus,
								},
							}
						}(),
					},
				},
			},
		},
	}
}
