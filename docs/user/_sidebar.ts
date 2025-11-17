export default [
  { text: 'Telemetry Pipeline API', link: './pipelines' },
  { text: 'Set Up the OTLP Input', link: './otlp-input' },
  {
    text: 'Collecting Logs', link: './collecting-logs/README', collapsed: true, items: [
      { text: 'Configure Application Logs', link: './collecting-logs/application-input' },
      { text: 'Configure Istio Access Logs', link: './collecting-logs/istio-support' },
    ]
  },
  {
    text: 'Collecting Traces', link: './collecting-traces/README', collapsed: true, items: [
      { text: 'Configure Istio Tracing', link: './collecting-traces/istio-support' },
    ]
  },
  {
    text: 'Collecting Metrics', link: './collecting-metrics/README', collapsed: true, items: [
      { text: 'Collect Prometheus Metrics', link: './collecting-metrics/prometheus-input' },
      { text: 'Collect Istio Metrics', link: './collecting-metrics/istio-input' },
      { text: 'Collect Runtime Metrics', link: './collecting-metrics/runtime-input' },
    ]
  },
  {
    text: 'Filtering and Processing Data', link: './filter-and-process/README', collapsed: true, items: [
      { text: 'Filter Logs', link: './filter-and-process/filter-logs' },
      { text: 'Filter Traces', link: './filter-and-process/filter-traces' },
      { text: 'Filter Metrics', link: './filter-and-process/filter-metrics' },
      { text: 'Transformation to OTLP Logs', link: './filter-and-process/transformation-to-otlp-logs' },
      { text: 'Automatic Data Enrichment', link: './filter-and-process/automatic-data-enrichment' },
      { text: 'Transform and Filter Telemetry Data with OTTL', link: './filter-and-process/ottl-transform-and-filter/README', collapsed: true, items: [
        { text: 'Transform with OTTL', link: './filter-and-process/ottl-transform-and-filter/ottl-transform' },
        { text: 'Filter with OTTL', link: './filter-and-process/ottl-transform-and-filter/ottl-filter' },
      ]},
    ]
  },
  {
    text: 'Integrate with your OTLP Backend', link: './integrate-otlp-backend/README', collapsed: true, items: [
      { text: 'Migrate Your LogPipeline from HTTP to OTLP Logs', link: './integrate-otlp-backend/migration-to-otlp-logs' },
    ]
  },
  { text: 'Monitor Pipeline Health', link: './monitor-pipeline-health' },
  { text: 'Troubleshooting the Telemetry Module', link: './troubleshooting' },
  {
    text: 'Architecture',
    link: './architecture/README',
    collapsed: true,
    items: [
      { text: 'Logs Architecture', link: './architecture/logs-architecture' },
      { text: 'Traces Architecture', link: './architecture/traces-architecture' },
      { text: 'Metrics Architecture', link: './architecture/metrics-architecture' },
      { text: 'Istio Integration', link: './architecture/istio-integration' },
    ]
  },
  {
    text: 'Integration Guides',
    link: './integration/README',
    collapsed: true,
    items: [
      { text: 'SAP Cloud Logging', link: './integration/sap-cloud-logging/README' },
      { text: 'Dynatrace', link: './integration/dynatrace/README' },
      { text: 'Prometheus, Grafana and Kiali', link: './integration/prometheus/README' },
      { text: 'Loki', link: './integration/loki/README' },
      { text: 'Jaeger', link: './integration/jaeger/README' },
      { text: 'Amazon CloudWatch', link: './integration/aws-cloudwatch/README' },
      { text: 'OpenTelemetry Demo App', link: './integration/opentelemetry-demo/README' },
      { text: 'Sample App', link: './integration/sample-app/README' },
    ]
  },
  {
    text: 'Resources',
    link: './resources/README',
    collapsed: true,
    items: [
      { text: 'Telemetry', link: './resources/01-telemetry' },
      { text: 'LogPipeline', link: './resources/02-logpipeline' },
      { text: 'TracePipeline', link: './resources/04-tracepipeline' },
      { text: 'MetricPipeline', link: './resources/05-metricpipeline' },
    ]
  },
  { text: 'Logs (Fluent Bit)', link: './02-logs' }
];
