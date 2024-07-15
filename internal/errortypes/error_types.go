package errortypes

type APIRequestFailed struct {
	Err error
}

func (a *APIRequestFailed) Error() string {
	return a.Err.Error()
}
