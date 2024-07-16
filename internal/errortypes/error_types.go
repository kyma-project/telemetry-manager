package errortypes

type APIRequestFailedError struct {
	Err error
}

func (a *APIRequestFailedError) Error() string {
	return a.Err.Error()
}
