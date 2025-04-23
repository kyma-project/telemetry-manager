# Migrate Ginkgo Tests to Go Testing PoC

This proof of concept demonstrates how to migrate Ginkgo tests to Go's built-in testing framework. The goal is to convert Ginkgo-style tests into the plain Go testing style while maintaining the same functionality and structure.

## Built-in testing E2E test example

[Service Name Enrichment](./_service_name_enrichment_test.go)

## Gomega

Gomega is a matcher library for Go that provides expressive assertions. Even though it is often used together with Ginkgo, it can be also used with the built-in `testing` package to write tests in a more readable way. This means that we can keep our Gomega matchers while migrating from Ginkgo to Go's testing framework. Note that in every test func, we must call `gomega.RegisterTestingT(t)` to register the testing.T instance with Gomega.

## Labels

In Ginkgo, labels are often used to group tests together. In the Go testing framework, we can use go test args to achieve a similar effect. It's a little bit less convenient, but it works.

## Cleanup

Built-in testing cleanup has a function `t.Cleanup()` that registers a function to be called when the test completes. We can use it similarly to `ginkgo.DeferCleanup()` to clean up resources after a test has run or failed.

## BeforeSuite / AfterSuite

In Ginkgo, `BeforeSuite` and `AfterSuite` are used to set up and tear down resources that are shared across all tests. In the Go testing framework, we can use `TestMain` to achieve similar functionality. The `TestMain` function is called before any tests are run, and we can use it to set up any necessary resources. After all tests have run, we can clean up those resources.Exit` call. We could also repeat the logic in each test.

> **NOTE:** 
> We must adapt the `testkit/suite` package so that it has no Gomega matchers and no `testenv` dependency. It must be usable by both Ginkgo and Go testing during the migration phase.
