package urlprovider

type URLProvider struct {
	metrics        string
	mockBackendURL map[string]string
	otlpPush       string
	metricPod      string
}

func New() *URLProvider {
	return &URLProvider{
		mockBackendURL: map[string]string{},
	}
}

func (p *URLProvider) SetOTLPPush(url string) *URLProvider {
	p.otlpPush = url
	return p
}

func (p *URLProvider) OTLPPush() string {
	return p.otlpPush
}
