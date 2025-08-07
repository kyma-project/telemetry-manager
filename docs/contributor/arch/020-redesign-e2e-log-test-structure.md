---
title: Redesign E2E Log Test Structure
status: Accepted
date: 2025-04-22
---

# 20. Redesign E2E Log Test Structure

## Context

### Current Test Coverage

Our current E2E test landscape suffers from the following issues:

- Sporadic test coverage: Only a subset of configurations are tested.
- No feature parity: Different pipeline configurations (like FluentBit vs. OpenTelemetry) are not tested against the same feature set. In the absence of comprehensive test cases, certain features have been inconsistently implemented.

It makes it hard to assert confidence across the key software components backing up LogPipelines.

### Target Structure

Going forward, we want to center our tests explicitly around the three core pieces of software that implement the LogPipeline:

- LogAgent (OTel Collector)
- LogGateway (OTel Collector)
- FluentBit

To ensure comprehensive, maintainable, and scalable test coverage, we will categorize tests as follows:

#### Shared FluentBit, LogAgent and LogGateway
- Single pipeline routing
- Multi pipeline routing
- Namespace selector
- mTLS tests
- Secret rotation
- Invalid configuration handling
- Resource checks
- Self-monitoring

#### Shared by FluentBit and LogAgent
- Container selector
- Drop labels
- Keep annotations
- Keep original body

#### Shared by LogAgent and LogGateway
- Kubernetes attribute enrichment
- Service name enrichment
- Label extraction
- Set observable time

#### Component-Specific Tests
- FluentBit:
  - Dedotting
  - Custom filters
  - Custom outputs
  - Custom parsers
- LogAgent:
  - TraceID/SpanID extraction
- LogGateway:
  - Manual gateway scaling

#### Miscellaneous
- Version conversion tests (for example, v1alpha1 → v1beta1)

Here’s a suggested file structure:

```bash
e2e
├── logs
│   ├── shared/ # Shared tests (all 3 components, FluentBit/LogAgent, LogGateway/LogAgent) implemented as table-driven
│   │   ├── namespace_selector_test.go # all 3 components
│   │   ├── container_selector_test.go # FluentBit/LogAgent (LogGateway does not support this)
│   │   ├── k8s_attr_enrichment.go # LogGateway/LogAgent (FluentBit does not support this)
│   │   ├── service_name_enrichment.go # LogGateway/LogAgent (FluentBit does not support this)
│   │   ├── mtls_test.go # all 3 components
│   │   ├── self_monitoring_test.go # all 3 components
│   │   └── validation_test.go # all 3 components
│   │
│   ├── fluentbit/
│   │   ├── dedotting_test.go
│   │   ├── custom_output_test.go
│   │
│   ├── logagent/
│   │   ├── trace_attribute_parser_test.go
│   │
│   ├── loggateway/
│   │   ├── otlp_push_test.go
│   │   └── manual_scaling_test.go
│   │
│   └── misc/
│   │   └── version_conversion_test.go
```

Shared tests will be implemented as table-driven tests, so we can easily add new test cases and configurations without duplicating code.

### Ginkgo to Built-in Go Testing Migration

Because we are going to rewrite our e2e tests, we decided to migrate from Ginkgo to the built-in Go testing framework. This will simplify the tests and make them easier to maintain. Ginkgo is a powerful testing framework, but it adds complexity and we don't really benefit from tests written in the BDD style. By using the built-in Go testing framework, we can write simpler and more straightforward tests that are easier to understand and maintain.

### Advantages of Built-in Go Testing:

* Simplicity
* No external dependencies or binaries needed
* Better support of table-driven tests
* No need to wrap tests in Ginkgo's `Describe` and `It` blocks, the descriptions are usually just blindly copy-pasted and have no value

Technical details are described in [Migrate Ginkgo Tests to Go Testing PoC](../pocs/ginkgo-to-go-testing/ginkgo-to-go-testing.md).

> **NOTE:**
> Changing all of the tests at once is not feasible. We will migrate the tests incrementally as we rewrite them. This means we will need two different Github Action jobs for Ginkgo and vanilla Go tests. When all tests are migrated, we will remove the Ginkgo job.
