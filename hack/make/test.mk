##@ Testing
.PHONY: test
test: manifests generate fmt vet tidy envtest ## Run tests.
	$(GINKGO) run ./test/testkit/matchers/...
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

.PHONY: check-coverage
check-coverage: go-test-coverage
	go test ./... -short -coverprofile=cover.out -covermode=atomic -coverpkg=./...
	$(GO_TEST_COVERAGE) --config=./.testcoverage.yml