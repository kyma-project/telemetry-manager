# End-to-end Tests

## Tracing

Tracing tests deploy a `TracePipeline`, which ships traces to a mock backend. The tests then use OpenTelemetry SDK to produce spans and send them to the trace collector. The mock backend is another OpenTelemetry collector with a file exporter and an OTLP receiver. All received spans are written to a JSON lines file (jsonlines.org). The received trace data is exposed using a webserver sidecar container and can be fetched and parsed by the tests.

![Tracing Tests Architecture](./assets/tracing-tests.svg)
