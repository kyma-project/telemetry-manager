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

func (p *URLProvider) SetMetrics(url string) *URLProvider {
	p.metrics = url
	return p
}

func (p *URLProvider) Metrics() string {
	return p.metrics
}

func (p *URLProvider) SetOTLPPush(url string) *URLProvider {
	p.otlpPush = url
	return p
}

func (p *URLProvider) OTLPPush() string {
	return p.otlpPush
}

func (p *URLProvider) SetMockBackendExport(backendName, url string) *URLProvider {
	p.mockBackendURL[backendName] = url

	return p
}

func (p *URLProvider) MockBackendExport(backendName string) string {
	return p.mockBackendURL[backendName]
}

func (p *URLProvider) SetMetricPodURL(url string) *URLProvider {
	p.metricPod = url
	return p
}

func (p *URLProvider) MetricPodURL() string {
	return p.metricPod
}
