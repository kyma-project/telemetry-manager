package kubeprep

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	// githubRepoOwner is the GitHub repository owner
	githubRepoOwner = "kyma-project"
	// githubRepoName is the GitHub repository name
	githubRepoName = "telemetry-manager"
)

// Config contains cluster preparation configuration
type Config struct {
	ManagerImage        string   // Required: telemetry manager container image
	LocalImage          bool     // Image is local (for k3d import and pull policy)
	InstallIstio        bool     // Install Istio before tests
	OperateInFIPSMode   bool     // Deploy manager in FIPS mode
	EnableExperimental  bool     // Enable experimental CRDs
	HelmValues          []string // Custom helm --set values (e.g., "additionalMetadata.labels.foo=bar")
	ChartPath           string   // Helm chart path/URL (empty = use local chart)
	DeployPrerequisites bool     // Deploy test prerequisites (default: true)
}

// Option is a functional option for configuring cluster setup
type Option func(*Config)

// WithHelmValues adds custom helm --set values
func WithHelmValues(values ...string) Option {
	return func(c *Config) {
		c.HelmValues = append(c.HelmValues, values...)
	}
}

// WithSkipDeployTestPrerequisites skips deploying test prerequisites.
// Use this when prerequisites are not needed or are deployed separately.
func WithSkipDeployTestPrerequisites() Option {
	return func(c *Config) {
		c.DeployPrerequisites = false
	}
}

// WithChartVersion sets the helm chart URL for deploying a specific version.
// Use this for upgrade tests to deploy an older version first.
// If chartURL is empty, fetches the latest release from GitHub.
func WithChartVersion(chartURL string) Option {
	return func(c *Config) {
		if chartURL == "" {
			// Fetch latest release URL
			url, err := getLatestReleaseChartURL()
			if err != nil {
				// Store error message - will be handled when Config is used
				c.ChartPath = fmt.Sprintf("error: %v", err)
				return
			}

			c.ChartPath = url
		} else {
			c.ChartPath = chartURL
		}
	}
}

// getLatestReleaseChartURL fetches the helm chart URL from the latest GitHub release
func getLatestReleaseChartURL() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", githubRepoOwner, githubRepoName)

	resp, err := http.Get(url) //nolint:gosec,noctx // GitHub API URL is safe
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to decode release response: %w", err)
	}

	// Find the helm chart asset (ends with .tgz)
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".tgz") {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("no helm chart (.tgz) found in release %s", release.TagName)
}

// isLocalImage detects if an image is local (not from a remote registry)
func isLocalImage(image string) bool {
	// Remote registries
	remoteRegistries := []string{
		"docker.pkg.dev",
		"gcr.io",
		"ghcr.io",
		"docker.io",
		"quay.io",
	}

	for _, registry := range remoteRegistries {
		if strings.Contains(image, registry) {
			return false
		}
	}

	return true
}

// IsLocalImage is a public wrapper for isLocalImage
func IsLocalImage(image string) bool {
	return isLocalImage(image)
}
