package e2e

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
)

const (
	// An API-server proxy port used by external clients to access in-cluster resources.
	apiPort = 6550
)

type httpsAuth struct {
	authToken       string
	TLSClientConfig *tls.Config
	apiPort         int
}

// newHTTPSAuth returns a provider for all HTTPS-related authentication information to be used
// for accessing in-cluster resources.
func newHTTPSAuth(authToken string, apiPort int) (auth httpsAuth, err error) {
	tlsConfig, err := newTLSConfig()
	if err != nil {
		return auth, fmt.Errorf("error creating an HTTPS transport instance: %w", err)
	}

	return httpsAuth{
		authToken:       authToken,
		TLSClientConfig: tlsConfig,
		apiPort:         apiPort,
	}, nil
}

func (a httpsAuth) TLSConfig() *tls.Config {
	return a.TLSClientConfig
}

func (a httpsAuth) Token() string {
	return "Bearer " + a.authToken
}

// URL composes a URL for an in-cluster resource.
func (a httpsAuth) URL(namespace, service, path string, port int) string {
	return fmt.Sprintf(
		`https://0.0.0.0:%d/api/v1/namespaces/%s/services/http:%s:%d/proxy/%s`,
		a.apiPort,
		namespace,
		service,
		port,
		strings.TrimLeft(path, "/"),
	)
}

// Get performs an HTTPS request to the in-cluster resource identifiable by URL.
func (a httpsAuth) Get(url string) (*http.Response, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: a.TLSConfig(),
	}}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", a.Token())

	return client.Do(req)
}

// newTLSConfig returns a TLS configuration that enables the use of mutual TLS.
func newTLSConfig() (*tls.Config, error) {
	return &tls.Config{
		InsecureSkipVerify: true,
	}, nil
}
