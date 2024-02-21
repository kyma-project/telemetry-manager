# Troubleshooting

## Pausing or Unpausing Reconciliations

You must pause reconciliations to be able to debug the pipelines or the Telemetry module. This is also useful to try out a different pipeline configuration or a different OTel configuration. To pause or unpause reconciliations, follow these steps:

1. Create an overriding `telemetry-override-config` ConfigMap in the manager's namespace.
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
  name: telemetry-override-config
data:
  override-config: |
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
```

The `global`, `tracing`, `logging`, `metrics`, and `telemetry` fields are optional.

**Caveats**
If you change the pipeline CR when the reconciliation is paused, these changes will not be applied immediately but in a periodic reconciliation cycle of one hour. To reconcile earlier, restart Telemetry Manager.

## Profiling Memory Problems

Telemetry Manager has pprof-based profiling activated and exposed on port 6060. Use port-forwarding to access the pprof endpoint. For more information, see the Go [pprof package documentation](https://pkg.go.dev/net/http/pprof).

## Scripts Don’t Work on Mackbook M1

For MacBook M1 users, some parts of the scripts may not work and they might see an error message like the following:
`Error: unsupported platform OS_TYPE: Darwin, OS_ARCH: arm64; to mitigate this problem set variable KYMA with the absolute path to kyma-cli binary compatible with your operating system and architecture. Stop.`

That's because Kyma CLI is not released for Apple Silicon users.

To fix it, install [Kyma CLI manually](https://github.com/kyma-project/cli#installation) and export the path to it.

   ```bash
   export KYMA=$(which kyma)
   ```
