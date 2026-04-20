package errortypes

type APIRequestFailedError struct {
	Err error
}

func (a *APIRequestFailedError) Error() string {
	return a.Err.Error()
}

// FlowHealthProbingFailedError is returned when the self-monitor Prometheus endpoint
// cannot be reached (DNS not ready, connection refused, etc.). Unlike most reconciler
// errors it must NOT feed the controller-runtime exponential backoff — callers should
// requeue with a short fixed delay instead.
type FlowHealthProbingFailedError struct {
	Err error
}

func (f *FlowHealthProbingFailedError) Error() string {
	return f.Err.Error()
}

func (f *FlowHealthProbingFailedError) Unwrap() error {
	return f.Err
}
