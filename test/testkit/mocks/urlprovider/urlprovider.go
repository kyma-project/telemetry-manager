package urlprovider

type URLProvider struct {
	mockBackendURL map[string]string
	otlpPush       string
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
