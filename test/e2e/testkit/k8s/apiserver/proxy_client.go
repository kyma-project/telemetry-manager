//go:build e2e

package apiserver

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

const (
	// An API-server proxy port used by external clients to access in-cluster resources.
	defaultAPIPort = 6550
)

type ProxyClient struct {
	bearerToken     string
	tlsClientConfig *tls.Config
	apiPort         int
}

// NewProxyClient returns a provider for all HTTPS-related authentication information to be used
// for accessing in-cluster resources.
func NewProxyClient(config *rest.Config) (*ProxyClient, error) {
	transportConfig, err := config.TransportConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create transport config: %w", err)
	}

	var tlsClientConfig *tls.Config
	tlsClientConfig, err = transport.TLSConfigFor(transportConfig)
	if tlsClientConfig == nil || err != nil {
		tlsClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	tlsClientConfig.InsecureSkipVerify = true

	return &ProxyClient{
		bearerToken:     transportConfig.BearerToken,
		tlsClientConfig: tlsClientConfig,
		apiPort:         defaultAPIPort,
	}, nil
}

func (a ProxyClient) TLSConfig() *tls.Config {
	return a.tlsClientConfig
}

func (a ProxyClient) Token() string {
	return "Bearer " + a.bearerToken
}

// ProxyURLForService composes a proxy url for a service.
func (a ProxyClient) ProxyURLForService(namespace, service, path string, port int) string {
	return fmt.Sprintf(
		`https://0.0.0.0:%d/api/v1/namespaces/%s/services/http:%s:%d/proxy/%s`,
		a.apiPort,
		namespace,
		service,
		port,
		strings.TrimLeft(path, "/"),
	)
}

// ProxyURLForPod composes a proxy url for a pod.
func (a ProxyClient) ProxyURLForPod(namespace, pod, path string, port int) string {
	return fmt.Sprintf(
		`https://0.0.0.0:%d/api/v1/namespaces/%s/pods/http:%s:%d/proxy/%s`,
		a.apiPort,
		namespace,
		pod,
		port,
		strings.TrimLeft(path, "/"),
	)
}

// Get performs an HTTPS request to the in-cluster resource identifiable by ProxyURLForService or ProxyURLForPod.
func (a ProxyClient) Get(proxyURL string) (*http.Response, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: a.tlsClientConfig,
	}}

	req, err := http.NewRequest(http.MethodGet, proxyURL, nil)
	if err != nil {
		return nil, err
	}

	if len(a.bearerToken) > 0 {
		req.Header.Set("Authorization", a.bearerToken)
	}

	return client.Do(req)
}
