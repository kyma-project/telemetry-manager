//go:build tools
// +build tools

package tools

// This file follows the recommendation at
// https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
// on how to pin tooling dependencies to a go.mod file.
// This ensures that all systems use the same version of tools in addition to regular dependencies.
import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/google/yamlfmt/cmd/yamlfmt"
	_ "github.com/kyma-project/kyma/hack/table-gen"
	_ "github.com/mikefarah/yq/v4"
	_ "github.com/onsi/ginkgo/v2/ginkgo"
	_ "github.com/vektra/mockery/v2"
	_ "github.com/vladopajic/go-test-coverage/v2"
	_ "golang.org/x/tools/cmd/stringer"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
	_ "sigs.k8s.io/kustomize/kustomize/v5"
)
