package objects

import (
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	istionetworkingv1 "istio.io/api/networking/v1"
	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VirtualService builds an Istio VirtualService with optional HTTP fault injection (delay / abort).
//
// For self-monitor and fault-backend tests, align abort status with exporter retry semantics:
//   - HTTP 400 Bad Request: non-retryable for both OTel Collector and Fluent Bit.
//   - HTTP 429 Too Many Requests: retryable for both.
//   - gRPC "INVALID_ARGUMENT": non-retryable for the OTel gRPC exporter; use this when the
//     target service communicates over gRPC so the exporter records send_failed rather than
//     a transport-level connection error.
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
	faultAbortGrpcStatus string
}

type Option func(*VirtualService)

func NewVirtualService(name, namespace, host string) *VirtualService {
	return &VirtualService{
		name:      name,
		namespace: namespace,
		host:      host,
	}
}

func (s *VirtualService) WithSourceLabel(sourceLabel map[string]string) *VirtualService {
	s.sourceLabel = sourceLabel
	return s
}

// WithFaultAbortPercentage sets the fraction of requests that receive an HTTP fault abort with httpStatus
// (e.g. http.StatusBadRequest for non-retryable, http.StatusTooManyRequests for retryable).
// Use WithFaultAbortGrpcStatus instead when the target service communicates over gRPC.
func (s *VirtualService) WithFaultAbortPercentage(percentage float64, httpStatus int32) *VirtualService {
	s.faultAbortPercentage = percentage
	s.faultAbortHttpStatus = httpStatus

	return s
}

// WithFaultAbortGrpcStatus sets the fraction of requests that receive a gRPC fault abort with the given
// gRPC status string (e.g. "INVALID_ARGUMENT" for non-retryable, "UNAVAILABLE" for retryable).
// Use this instead of WithFaultAbortPercentage when the target service communicates over gRPC,
// so the OTel gRPC exporter records send_failed rather than a transport-level connection error.
func (s *VirtualService) WithFaultAbortGrpcStatus(percentage float64, grpcStatus string) *VirtualService {
	s.faultAbortPercentage = percentage
	s.faultAbortGrpcStatus = grpcStatus

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

							abort := &istionetworkingv1.HTTPFaultInjection_Abort{
								Percentage: &istionetworkingv1.Percent{Value: s.faultAbortPercentage},
							}

							if s.faultAbortGrpcStatus != "" {
								abort.ErrorType = &istionetworkingv1.HTTPFaultInjection_Abort_GrpcStatus{
									GrpcStatus: s.faultAbortGrpcStatus,
								}
							} else {
								abort.ErrorType = &istionetworkingv1.HTTPFaultInjection_Abort_HttpStatus{
									HttpStatus: s.faultAbortHttpStatus,
								}
							}

							return abort
						}(),
					},
				},
			},
		},
	}
}
