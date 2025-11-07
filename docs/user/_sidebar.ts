export default [
  { text: 'Telemetry Pipeline API', link: '/telemetry-manager/user/pipelines.md' },
  { text: 'Set Up the OTLP Input', link: '/telemetry-manager/user/collecting-logs/README.md' },
  {
    text: 'Collecting Logs', link: '/telemetry-manager/user/collecting-logs/README.md', collapsed: true, items: [
      { text: 'Configure Application Logs', link: '/telemetry-manager/user/collecting-logs/application-input.md' },
      { text: 'Configure Istio Access Logs', link: '/telemetry-manager/user/collecting-logs/istio-support.md' },
    ]
  },
  {
    text: 'Collecting Traces', link: '/telemetry-manager/user/collecting-traces/README.md', collapsed: true, items: [
      { text: 'Configure Istio Tracing', link: '/telemetry-manager/user/collecting-traces/istio-support.md' },
    ]
  },
  {
    text: 'Collecting Metrics', link: '/telemetry-manager/user/collecting-metrics/README.md', collapsed: true, items: [
      { text: 'Collect Prometheus Metrics', link: '/telemetry-manager/user/collecting-metrics/prometheus-input.md' },
      { text: 'Collect Istio Metrics', link: '/telemetry-manager/user/collecting-metrics/istio-input.md' },
      { text: 'Collect Runtime Metrics', link: '/telemetry-manager/user/collecting-metrics/runtime-input.md' },
    ]
  },
  {
    text: 'Filtering and Processing Data', link: '/telemetry-manager/user/filter-and-process/README.md', collapsed: true, items: [
      { text: 'Filter Logs', link: '/telemetry-manager/user/filter-and-process/filter-logs.md' },
      { text: 'Filter Traces', link: '/telemetry-manager/user/filter-and-process/filter-traces.md' },
      { text: 'Filter Metrics', link: '/telemetry-manager/user/filter-and-process/filter-metrics.md' },
      { text: 'Transformation to OTLP Logs', link: '/telemetry-manager/user/filter-and-process/transformation-to-otlp-logs.md' },
      { text: 'Automatic Data Enrichment', link: '/telemetry-manager/user/filter-and-process/automatic-data-enrichment.md' },
      { text: 'Transform and Filter Telemetry Data with OTTL', link: '/telemetry-manager/user/filter-and-process/ottl-transform-and-filter/README.md', collapsed: true, items: [
        { text: 'Transform with OTTL', link: '/telemetry-manager/user/filter-and-process/ottl-transform-and-filter/ottl-transform.md' },
        { text: 'Filter with OTTL', link: '/telemetry-manager/user/filter-and-process/ottl-transform-and-filter/ottl-filter.md' },
      ]},
    ]
  },
  {
    text: 'Integrate with your OTLP Backend', link: '/telemetry-manager/user/integrate-otlp-backend/README.md', collapsed: true, items: [
      { text: 'Migrate Your LogPipeline from HTTP to OTLP Logs', link: '/telemetry-manager/user/integrate-otlp-backend/migration-to-otlp-logs.md' },
    ]
  },
  { text: 'Monitor Pipeline Health', link: '/telemetry-manager/user/monitor-pipeline-health.md' },
  { text: 'Troubleshooting the Telemetry Module', link: '/telemetry-manager/user/troubleshooting.md' },
  {
    text: 'Architecture',
    link: '/telemetry-manager/user/architecture/README.md',
    collapsed: true,
    items: [
      { text: 'Logs Architecture', link: '/telemetry-manager/user/architecture/logs-architecture.md' },
      { text: 'Traces Architecture', link: '/telemetry-manager/user/architecture/traces-architecture.md' },
      { text: 'Metrics Architecture', link: '/telemetry-manager/user/architecture/metrics-architecture.md' },
      { text: 'Istio Integration', link: '/telemetry-manager/user/architecture/istio-integration.md' },
    ]
  },
  {
    text: 'Integration Guides',
    link: '/telemetry-manager/user/integration/README.md',
    collapsed: true,
    items: [
      { text: 'SAP Cloud Logging', link: '/telemetry-manager/user/integration/sap-cloud-logging/README.md' },
      { text: 'Dynatrace', link: '/telemetry-manager/user/integration/dynatrace/README.md' },
      { text: 'Prometheus, Grafana and Kiali', link: '/telemetry-manager/user/integration/prometheus/README.md' },
      { text: 'Loki', link: '/telemetry-manager/user/integration/loki/README.md' },
      { text: 'Jaeger', link: '/telemetry-manager/user/integration/jaeger/README.md' },
      { text: 'Amazon CloudWatch', link: '/telemetry-manager/user/integration/aws-cloudwatch/README.md' },
      { text: 'OpenTelemetry Demo App', link: '/telemetry-manager/user/integration/opentelemetry-demo/README.md' },
      { text: 'Sample App', link: '/telemetry-manager/user/integration/sample-app/README.md' },
    ]
  },
  {
    text: 'Resources',
    link: '/telemetry-manager/user/resources/README.md',
    collapsed: true,
    items: [
      { text: 'Telemetry', link: '/telemetry-manager/user/resources/01-telemetry.md' },
      { text: 'LogPipeline', link: '/telemetry-manager/user/resources/02-logpipeline.md' },
      { text: 'TracePipeline', link: '/telemetry-manager/user/resources/04-tracepipeline.md' },
      { text: 'MetricPipeline', link: '/telemetry-manager/user/resources/05-metricpipeline.md' },
    ]
  },
  { text: 'Logs (Fluent Bit)', link: '/telemetry-manager/user/02-logs.md' }
];
