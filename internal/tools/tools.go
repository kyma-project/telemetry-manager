//go:build tools
// +build tools

package tools

// This file follows the recommendation at
// https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
// on how to pin tooling dependencies to a go.mod file.
// This ensures that all systems use the same version of tools in addition to regular dependencies.
import (
	_ "github.com/g4s8/envdoc"
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	_ "github.com/google/yamlfmt/cmd/yamlfmt"
	_ "github.com/hairyhenderson/gomplate/v4/cmd/gomplate"
	_ "github.com/itchyny/gojq/cmd/gojq"
	_ "github.com/k3d-io/k3d/v5"
	_ "github.com/kyma-project/kyma/hack/table-gen"
	_ "github.com/mikefarah/yq/v4"
	_ "github.com/vektra/mockery/v3"
	_ "github.com/vladopajic/go-test-coverage/v2"
	_ "github.com/yeya24/promlinter/cmd/promlinter"
	_ "golang.org/x/tools/cmd/stringer"
	_ "gotest.tools/gotestsum"
	_ "helm.sh/helm/v4/cmd/helm"
	_ "k8s.io/code-generator/cmd/conversion-gen"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
