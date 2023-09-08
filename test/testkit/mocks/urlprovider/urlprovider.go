package urlprovider

type URLProvider struct {
	metrics   string
	pipelines map[string]map[string]string
	otlpPush  string
	metricPod string
}

func New() *URLProvider {
	return &URLProvider{
		pipelines: map[string]map[string]string{},
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

func (p *URLProvider) SetMockBackendExport(url, backendName string) *URLProvider {
	if p.pipelines[backendName] == nil {
		p.pipelines[backendName] = map[string]string{}
	}

	p.pipelines[backendName]["mockBackendExport"] = url

	return p
}

func (p *URLProvider) MockBackendExport(backendName string) string {
	return p.pipelines[backendName]["mockBackendExport"]
}

func (p *URLProvider) SetMetricPodURL(url string) *URLProvider {
	p.metricPod = url
	return p
}

func (p *URLProvider) MetricPodURL() string {
	return p.metricPod
}
