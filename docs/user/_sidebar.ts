export default [
  { text: 'Telemetry Pipeline API', link: './pipelines.md' },
  { text: 'Set Up the OTLP Input', link: './otlp-input.md' },
  {
    text: 'Collecting Logs', link: './collecting-logs/README.md', collapsed: true, items: [
      { text: 'Configure Application Logs', link: './collecting-logs/application-input.md' },
      { text: 'Configure Istio Access Logs', link: './collecting-logs/istio-support.md' },
    ]
  },
  {
    text: 'Collecting Traces', link: './collecting-traces/README.md', collapsed: true, items: [
      { text: 'Configure Istio Tracing', link: './collecting-traces/istio-support.md' },
    ]
  },
  {
    text: 'Collecting Metrics', link: './collecting-metrics/README.md', collapsed: true, items: [
      { text: 'Collect Prometheus Metrics', link: './collecting-metrics/prometheus-input.md' },
      { text: 'Collect Istio Metrics', link: './collecting-metrics/istio-input.md' },
      { text: 'Collect Runtime Metrics', link: './collecting-metrics/runtime-input.md' },
    ]
  },
  {
    text: 'Filtering and Processing Data', link: './filter-and-process/README.md', collapsed: true, items: [
      { text: 'Filter Logs', link: './filter-and-process/filter-logs.md' },
      { text: 'Filter Traces', link: './filter-and-process/filter-traces.md' },
      { text: 'Filter Metrics', link: './filter-and-process/filter-metrics.md' },
      { text: 'Transformation to OTLP Logs', link: './filter-and-process/transformation-to-otlp-logs.md' },
      { text: 'Automatic Data Enrichment', link: './filter-and-process/automatic-data-enrichment.md' },
      { text: 'Transform and Filter Telemetry Data with OTTL', link: './filter-and-process/ottl-transform-and-filter/README.md', collapsed: true, items: [
        { text: 'Transform with OTTL', link: './filter-and-process/ottl-transform-and-filter/ottl-transform.md' },
        { text: 'Filter with OTTL', link: './filter-and-process/ottl-transform-and-filter/ottl-filter.md' },
      ]},
    ]
  },
  {
    text: 'Integrate with your OTLP Backend', link: './integrate-otlp-backend/README.md', collapsed: true, items: [
      { text: 'Migrate Your LogPipeline from HTTP to OTLP Logs', link: './integrate-otlp-backend/migration-to-otlp-logs.md' },
    ]
  },
  { text: 'Monitor Pipeline Health', link: './monitor-pipeline-health.md' },
  { text: 'Troubleshooting the Telemetry Module', link: './troubleshooting.md' },
  {
    text: 'Architecture',
    link: './architecture/README.md',
    collapsed: true,
    items: [
      { text: 'Logs Architecture', link: './architecture/logs-architecture.md' },
      { text: 'Traces Architecture', link: './architecture/traces-architecture.md' },
      { text: 'Metrics Architecture', link: './architecture/metrics-architecture.md' },
      { text: 'Istio Integration', link: './architecture/istio-integration.md' },
    ]
  },
  {
    text: 'Integration Guides',
    link: './integration/README.md',
    collapsed: true,
    items: [
      { text: 'SAP Cloud Logging', link: './integration/sap-cloud-logging/README.md' },
      { text: 'Dynatrace', link: './integration/dynatrace/README.md' },
      { text: 'Prometheus, Grafana and Kiali', link: './integration/prometheus/README.md' },
      { text: 'Loki', link: './integration/loki/README.md' },
      { text: 'Jaeger', link: './integration/jaeger/README.md' },
      { text: 'Amazon CloudWatch', link: './integration/aws-cloudwatch/README.md' },
      { text: 'OpenTelemetry Demo App', link: './integration/opentelemetry-demo/README.md' },
      { text: 'Sample App', link: './integration/sample-app/README.md' },
    ]
  },
  {
    text: 'Resources',
    link: './resources/README.md',
    collapsed: true,
    items: [
      { text: 'Telemetry', link: './resources/01-telemetry.md' },
      { text: 'LogPipeline', link: './resources/02-logpipeline.md' },
      { text: 'TracePipeline', link: './resources/04-tracepipeline.md' },
      { text: 'MetricPipeline', link: './resources/05-metricpipeline.md' },
    ]
  },
  { text: 'Logs (Fluent Bit)', link: './02-logs.md' }
];
