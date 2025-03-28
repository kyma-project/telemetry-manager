<!-- The table below was generated automatically -->
<!-- Please do not edit it directly. If you want to change it, please update the template and regenerate the file with make target  update-metrics-docs -->
# Metrics Emitted by Kyma Telemetry Manager

| Metric                                                          | Description                                                                                   |
|-----------------------------------------------------------------|:----------------------------------------------------------------------------------------------|
{{- range (ds "telemetry") }}
| **{{.Name}}** | {{.Help}} |
{{- end }}