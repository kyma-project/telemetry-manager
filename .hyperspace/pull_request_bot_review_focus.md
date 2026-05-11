# Telemetry Manager Review Focus

## Project Structure Rules

1. Controllers in `controllers/telemetry/` must only serve as composition roots (dependency injection, watcher setup) — business logic belongs in `internal/reconciler/`
2. Configuration builders live in `internal/otelcollector/config/` or `internal/fluentbit/config/` — no config construction elsewhere
3. Kubernetes resources are generated via `ApplierDeleters` in `internal/resources/` — avoid inline resource construction in reconcilers
4. Mocks are generated with mockery (`.mockery.yml`) — do not hand-write mocks

## Review Priorities

- Verify reconciler changes do not skip error handling for Kubernetes API calls
- Flag missing or incorrect RBAC annotations when new resource types are accessed
- Check that new CRD fields have corresponding DeepCopy generation (via `make generate`)
- Ensure new environment variables added to the manager are declared in `main.go` envConfig struct
- Verify that pipeline validation logic lives in `internal/validators/` not in controllers
- Flag direct use of `client.Update` where `client.Patch` (status subresource) should be used for status updates
- Check that golden file tests are updated (`make update-golden-files`) when config builders change
- Verify that e2e and integration test helpers are placed in `test/testkit/` not duplicated inline in test files; unit-test helpers in `internal/utils/test/` are also acceptable

## Documentation Checks

- New or changed environment variables must be documented (e.g. in `docs/` or the relevant reference page) — flag any that are only declared in code without a corresponding docs update
- New or changed manager flags (command-line flags passed to the manager binary) must be documented — check `docs/` for a flags or configuration reference
- New or changed self-monitoring metrics (Prometheus metrics exposed by the manager) must be documented — check `docs/user/` for a metrics reference page and flag missing entries
