//go:build e2e

package mocks

type URLProvider struct {
	metrics   string
	pipelines map[int]map[string]string
}

func NewURLProvider() *URLProvider {
	return &URLProvider{
		pipelines: map[int]map[string]string{},
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
	return p.SetOTLPPushAt(url, 0)
}

func (p *URLProvider) OTLPPush() string {
	return p.OTLPPushAt(0)
}

func (p *URLProvider) SetOTLPPushAt(url string, idx int) *URLProvider {
	if p.pipelines[idx] == nil {
		p.pipelines[idx] = map[string]string{}
	}

	p.pipelines[idx]["otlpPush"] = url

	return p
}

func (p *URLProvider) OTLPPushAt(idx int) string {
	return p.pipelines[idx]["otlpPush"]
}

func (p *URLProvider) SetMockBackendExport(url string) *URLProvider {
	return p.SetMockBackendExportAt(url, 0)
}

func (p *URLProvider) MockBackendExport() string {
	return p.MockBackendExportAt(0)
}

func (p *URLProvider) SetMockBackendExportAt(url string, idx int) *URLProvider {
	if p.pipelines[idx] == nil {
		p.pipelines[idx] = map[string]string{}
	}

	p.pipelines[idx]["mockBackendExport"] = url

	return p
}

func (p *URLProvider) MockBackendExportAt(idx int) string {
	return p.pipelines[idx]["mockBackendExport"]
}
