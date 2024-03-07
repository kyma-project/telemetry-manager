package alerts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	selfMonitorAPIURL = "http://telemetry-self-monitor.kyma-system:80"
	clientTimeout     = 10 * time.Second
)

type Client struct {
	promAPI       promv1.API
	clientTimeout time.Duration
}

func NewClient() (*Client, error) {
	client, err := api.NewClient(api.Config{
		Address: selfMonitorAPIURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	return &Client{
		promAPI:       promv1.NewAPI(client),
		clientTimeout: clientTimeout,
	}, nil
}

func (c *Client) AllDataDropped(ctx context.Context, pipelineName string) (bool, error) {
	alerts, err := c.retrieveAlerts(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	exporterSentFiring := hasFiringAlert(alerts, "GatewayExporterSent", pipelineName)
	exporterDroppedFiring := hasFiringAlert(alerts, "GatewayExporterDropped", pipelineName)
	exporterEnqueueFailedFiring := hasFiringAlert(alerts, "GatewayExporterEnqueueFailed", pipelineName)

	return !exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring), nil
}

func (c *Client) retrieveAlerts(ctx context.Context) ([]promv1.Alert, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.clientTimeout)
	defer cancel()

	result, err := c.promAPI.Alerts(childCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to query Prometheus alerts: %w", err)
	}

	return result.Alerts, nil
}

func hasFiringAlert(alerts []promv1.Alert, alertNamePrefix, pipelineName string) bool {
	for _, alert := range alerts {
		isFiring := alert.State == promv1.AlertStateFiring
		hasMatchingName := hasMatchingLabelValue(alert, "alertname", alertNamePrefix)
		hasMatchingExporter := hasMatchingLabelValue(alert, "exporter", pipelineName)

		if isFiring && hasMatchingName && hasMatchingExporter {
			return true
		}
	}

	return false
}

func hasMatchingLabelValue(alert promv1.Alert, labelName, labelValue string) bool {
	v, ok := alert.Labels[model.LabelName(labelName)]
	return ok && strings.Contains(string(v), labelValue)
}
