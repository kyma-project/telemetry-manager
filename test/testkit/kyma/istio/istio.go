package modules

import (
	"context"
	"github.com/google/go-github/github"
)

type Istio struct {
	name string
}

func NewIstioModule(name string) *Istio {
	return &Istio{
		name: name,
	}
}

func DeployModule() {
	releases, err := getLatestRelease()
	if err != nil {
		return
	}
}

func (i *Isito) istioCR() *string {
	release, err := getLatestRelease()
	if err != nil {
		return nil
	}
	for _, asset := range release.Assets {
		if *asset.Name == "istio-default-cr.yaml" {
			return asset.BrowserDownloadURL
		}
	}
	return nil
}

func getLatestRelease() (*github.RepositoryRelease, error) {
	ctx := context.TODO()
	client := github.NewClient(nil)
	opt := &github.ListOptions{Page: 1, PerPage: 10}
	releases, _, err := client.Repositories.ListReleases(ctx, "kyma-project", "istio", opt)
	if err != nil {
		return nil, err
	}
	return releases[0], nil
}
