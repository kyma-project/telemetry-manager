# End-to-end Tests

## Tracing

Tracing tests deploy a `TracePipeline`, which ships traces to a mock backend. The tests then use OpenTelemetry SDK to produce spans and send them to the trace collector. The mock backend is another OpenTelemetry collector with a file exporter and an OTLP receiver. All received spans are written to a JSON lines file (jsonlines.org). The received trace data is exposed using a webserver sidecar container and can be fetched and parsed by the tests.

![Tracing Tests Architecture](./assets/tracing-tests.svg)


### Exposing Internal Services

In order to send and receive, and analyze traces the test has to expose internal in-cluster services. Currently, it's done using `NodePort` services and k3d host port mapping. 
This approach is very easy to implement. However it has a few major drawbacks and needs to be rethought:
* `NodePort` service exposure will only work on k3d and not on public cloud (Gardener)
* One has to always keep a mind map with port map in their head. For every new internal service, a few places in the code have to be updated

#### Port Mapping

