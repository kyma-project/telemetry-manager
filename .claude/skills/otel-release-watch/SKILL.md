---
name: otel-release-watch
description: >-
  Assess how a new OpenTelemetry Collector release (core + contrib) affects this
  module, and track cross-cutting OTel ecosystem developments (OTEPs, semconv,
  signal stabilizations). Use when bumping the collector dependency/image, or for
  a periodic "what changed in OTel that we should care about" review.
---

# OTel release watch

Telemetry Manager builds a custom OTel Collector distribution and generates its
config. When the collector version moves, two very different kinds of breakage
are possible, and they must be assessed separately:

1. **Generated config** (`internal/otelcollector/config/**`) — we emit plain
   YAML/OTTL **strings**. Only affected by *config-surface* changes: removed
   config keys, renamed components, changed defaults, removed/renamed OTTL
   functions, or stricter OTTL parsing. Low-to-medium risk; verified by golden
   files.
2. **Go-compiled code** (`internal/validators/ottl/**`, and the custom
   distribution's own receivers/processors like `kymastatsreceiver` and
   `istio_noise_filter`) — compiled against the collector **Go API**. Affected by
   *API-surface* changes: unexported fields, changed signatures, removed helpers,
   moved packages. This is where a version bump most often **fails to compile**.

Do not conflate them. A changelog entry marked "breaking" is frequently a Go-API
break that never touches our generated config — and vice versa.

## When to use

- Bumping `go.opentelemetry.io/collector/*`, `.../opentelemetry-collector-contrib/*`,
  or the `kyma-otel-collector` image.
- A periodic ecosystem scan for OTEPs / semconv / signal-stability changes worth
  tracking (e.g. K8s semconv stabilization, OTTL 1.0, Profiles, declarative config).

## Step 1 — Establish our surface area

Never analyze a changelog in the abstract. First pin down exactly what we use.

```bash
# Current pinned versions (core is 1.x, contrib is 0.x — they differ)
grep -E "go.opentelemetry.io/collector|opentelemetry-collector-contrib" go.mod

# Contrib components we depend on directly
grep -E "opentelemetry-collector-contrib" go.mod | grep -Ei "receiver|processor|exporter|extension|connector|pkg/ottl"

# Collector image / test images
grep -Ei "OTEL_COLLECTOR|TELEMETRYGEN" .env
```

Known surface (keep this current):

- **Contrib direct:** `pkg/ottl`, `processor/filterprocessor`, `processor/transformprocessor`
  (transitive: `internal/filter`, `internal/coreinternal`).
- **Receivers we configure:** `prometheusreceiver`, `kubeletstatsreceiver`,
  `k8sclusterreceiver`, `k8sattributesprocessor`; plus our own `kymastatsreceiver`.
- **Core:** `otlp` receiver/exporter, `batch`, `memory_limiter`, `resource`,
  `attributes`, `routing`/`forward` connectors, `pdata`.
- **Highest-usage OTTL:** `set()` (~1.5k occurrences), `resource.attributes`,
  `IsMatch`, `delete_key`, transform/filter `where`/conditions. OTTL is our
  single largest and riskiest surface — treat OTTL changes as top priority.
- **OTTL is exposed to users** via `TransformSpec`/`FilterSpec` and validated in
  the admission webhook (`internal/validators/ottl/`, entered from
  `webhook/utils/transformfilter.go`). This validator is compiled against the
  OTTL Go API — see Step 3.

## Step 2 — Diff the changelogs (core AND contrib, every intermediate version)

Fetch the changelogs and read **every** version between our current pin and the
target — not just the endpoints. Prefer the raw files:

- Contrib: `https://raw.githubusercontent.com/open-telemetry/opentelemetry-collector-contrib/main/CHANGELOG.md`
  and `CHANGELOG-API.md` (API breaks live here).
- Core: `https://raw.githubusercontent.com/open-telemetry/opentelemetry-collector/main/CHANGELOG.md`
  and `CHANGELOG-API.md`.

For each entry that touches our surface, classify it:

- 🔴 **BREAKING** — config/API change requiring action (removed key, renamed
  component, unexported field, changed signature, removed feature gate).
- 🟡 **BEHAVIORAL** — default changed / feature gate promoted to on-by-default.
  No code change, but output or timing shifts. **Re-check golden files + E2E.**
- 🟢 **FEATURE** — new opt-in capability.
- ⚪ bugfix / deprecation notice.

Watch especially for: **feature-gate promotions and removals** (a gate promoted
to stable is then *removed* — passing it on the command line becomes a fatal
error), **OTTL** function/grammar/parser changes, **prometheus/kubeletstats/
k8sattributes** behavioral defaults, and **exporterhelper** queue/batch changes.

## Step 3 — Verify each candidate against the repo (turn "might" into "does")

A changelog says what *could* matter; the repo says what *does*. Grep before you
conclude:

```bash
# Removed gates / APIs / renamed components — should return NOTHING if safe
grep -rEn "pdatautil|Base64Decode|queuebatch|PanicDuplicateName|enableOTelColContext|defaultErrorModeIgnore" \
  --include="*.go" --include="*.yaml" . | grep -v /vendor/ | grep -v _test

# The OTTL validator's Go-API usage — the most bump-fragile file in the repo.
# e.g. ParserCollection.Settings/ErrorMode were unexported → field access must
# become method calls (pc.Settings -> pc.Settings()). Default*Functions() names
# have churned. Verify every symbol still exists with its current signature.
grep -rEn "\.Settings\b|Default[A-Za-z]*Functions|StandardFuncs|NewParser|ParseStatements|ParseConditions" \
  internal/validators/ottl/

# Behavioral checks: is an at-risk pattern guarded? (e.g. set(x, y) with a
# where-guard is safe under ottl.set.allowNil going on-by-default)
grep -rEn "JoinWithWhere|IsNotNil|set\(" internal/otelcollector/config/common/processor_builders.go
```

Confirm ambiguous claims against the **current upstream source** (WebFetch the
`.go` file), not just the changelog summary — dates and "v1.0.0" labels in
release blogs are often approximate; feature-gate stage (alpha/beta/stable) and
struct field visibility are the ground truth.

## Step 4 — Report, split by risk

Produce a table ranked by severity. For each item: component, version it landed
in, one-line description, classification, and **the concrete repo impact you
verified** (file:line, "safe because …", or "won't compile — change X to Y").
Keep generated-config items and Go-compile items in separate sections.

Always end with the mechanical follow-ups:

- `make update-golden-files` and diff — behavioral OTTL/prometheus/semconv
  changes surface here.
- `make build` + validator tests — catches Go-API breaks in
  `internal/validators/ottl/`.
- Grep the **`kyma-otel-collector` distribution repo** (separate from this one)
  for removed-gate command-line flags and removed-package imports — that is where
  our custom `kymastatsreceiver` / `istio_noise_filter` compile.

## Step 5 (optional) — Ecosystem scan

For a periodic "what's coming in OTel" review, check, with status + a one-line
"why it matters to a collector-deploying operator":

- **OTEPs:** `github.com/open-telemetry/opentelemetry-specification/tree/main/oteps`
  and open OTEP PRs (e.g. 4738 telemetry-policy, 4485 complex attributes).
- **Signal stability:** `opentelemetry.io/docs/specs/status/` — Logs, Profiles
  (4th signal, alpha), Entities.
- **Semantic conventions:** Kubernetes semconv stabilization, `service.*`/`k8s.*`
  attribute changes — these can rename attributes we depend on.
- **Collector direction:** OTTL 1.0 progress, declarative config schema, OpAMP.

Note the module already emits **v1 (singular)** K8s label naming
(`k8s.pod.label.<key>`) via explicit `TagName`s, so the k8sattributes v0→v1
convention flip (`EmitV1K8sConventions`/`DontEmitV0K8sConventions`, beta/on since
contrib 0.161.0) is a no-op for us — but re-verify if we ever drop the explicit
`TagName`.
