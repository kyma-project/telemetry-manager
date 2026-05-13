# Troubleshooting

## Pausing or Unpausing Reconciliations

You must pause reconciliations to be able to debug the pipelines or the Telemetry module. This is also useful to try out a different pipeline configuration or a different OTel configuration. To pause or unpause reconciliations, follow these steps:

1. Create an overriding `telemetry-overrides` ConfigMap in the manager's namespace.
2. Perform debugging operations.
3. Remove the created ConfigMap.
4. To reset the debug actions, perform a restart of Telemetry Manager.

   ```bash
   kubectl rollout restart deployment telemetry-manager
   ```

Here is an example of such a ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: telemetry-overrides
data:
  overrides: |
    global:
      logLevel: debug
    tracing:
      paused: true
    logging:
      paused: true
    metrics:
      paused: true
    telemetry:
      paused: true
    otlpGateway:
      enabled: false
```

All fields are optional.

**Caveats**
If you change the pipeline CR when the reconciliation is paused, these changes will not be applied immediately but in a periodic reconciliation cycle of one hour. To reconcile earlier, restart Telemetry Manager.

## Profiling Memory Problems

Telemetry Manager has pprof-based profiling activated and exposed on port 6060. Use port-forwarding to access the pprof endpoint. For more information, see the Go [pprof package documentation](https://pkg.go.dev/net/http/pprof).
