# Testing Strategy
​
This document summarises the software development testing life cycle activities and artefacts for the telemetry module.
​
## Roles and Responsibilities
​
Software Testing Life Cycle phases:
​
| Phase                                 | When                                            | How                                                          | Result                                                       |
| ------------------------------------- | ----------------------------------------------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| Requirement analysis                  | Sprint planning, grooming and triaging meetings | Brainstorm the feature implementation, design and its implications on the testability | A clarified implementation design with a focus on testability, the acceptance tests criteria are collected, and the testing surface is defined as a part of the story |
| Test planning, design and development | Development phase during the sprint             | The new functionality is covered with sufficient Unit, Integration and Acceptance-tests. | The unit and integration test suites are augmented, and new tests are integrated into the CI pipeline. All new functionality is covered with an Acceptance test. |
| Test execution                        | A part of the CI process                        |                                                              |                                                              |
​
The roles and responsibilities during the STLC:
​
| Role        | Responsibilities                                             | Performed by     |
| ----------- | ------------------------------------------------------------ | ---------------- |
| `PO`        | Define acceptance criteria for the user stories, assess risks | `PO`             |
| `DEV`       | Implementation of tests for new functionality, an extension of the test suites, adherence to the test coverage expectations | development team |
| `ARCHITECT` | Devise system design with a focus on testability             | team-shared role |
| `QA`        | Define the testing coverage for each story, and ensure the test suite is delivered along with each new piece of functionality. | team-shared role |
​
## Testing Levels
​
### Functional Tests
​
`Unit` and `Env`-tests follow the go convention and reside next to the code they are testing. The unit tests and integration tests are a part of one test suite.
​
| Test suite                                                   | Testing level                         | Purpose                                                      |
| ------------------------------------------------------------ | ------------------------------------- | ------------------------------------------------------------ |
| Unit (located with the individual source files)              | Unit                                  | It tests the units in complete isolation. This test suite assesses the implementation correctness of the units of business logic. |
| Env-tests (located with the individual source files)         | Integration  (low-level)              | It tests the behaviour of the Telemetry Controller in integration with a k8s API server replaced with a test double. This test suite assesses the integration correctness of the controller. |
| [E2E](/test/e2e) | Acceptance / Integration (high-level) | It tests the usability scenarios of the Telemetry Controller in a cluster. This test suite assesses the functional correctness of the controller. |
​
### Non-functional Tests
​
| Type                                                         | Automation | Frequency                               | Results          |
| ------------------------------------------------------------ | ---------- | --------------------------------------- | ---------------- |
| Release testing | Manual     | Regularly before each release           | Manual tests     |
| Performance tests | Manual     | Ad hoc on a noticeable component change | Tests repository |
| Security tests (realized SAP internal) | Automated  |                                         |                  |
​
### Source Code Quality
​
The source code quality is maintained using a static code analysis provided by [golangci-lint](./governance.md).
​
The aspects analysed currently by the [configured linters](./governance.md#linters-in-action):
​
- [x] Adherence to the code style standards
- [x] Code semantics
- [ ] Module's dependencies management
- [ ] Codebase cognitive and cyclomatic complexity
​
## Test Deliverables
​
All testing-related deliverables except for the `Release Testing` report and `Performance Tests` results are integrated as stages of the Continuous Integration pipeline. 
​
The automated test reports are available via these links:
​
* [Unit and Integration test suite](https://status.build.kyma-project.io/?repo=kyma-project%2Ftelemetry-manager&job=pull-telemetry-manager-unit-test) (the current code coverage is a part of this report too)
* [E2E test suite](https://status.build.kyma-project.io/?repo=kyma-project%2Ftelemetry-manager&job=pull-telemetry-manager-e2e-test)
* [Source-code linting reports](https://status.build.kyma-project.io/?repo=kyma-project%2Ftelemetry-manager&job=pull-telemetry-manager-lint)
​