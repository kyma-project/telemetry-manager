package webhook

import (
	"context"
	"encoding/json"
	"github.com/go-logr/logr"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"io"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"slices"
	"strings"
)

type Handler struct {
	c           client.Reader
	subscribers map[string]chan<- event.GenericEvent
	logger      logr.Logger
}

type Option = func(*Handler)

func WithLogger(logger logr.Logger) Option {
	return func(h *Handler) {
		h.logger = logger
	}
}

func WithMetricPipelineSubscriber(subscriber chan<- event.GenericEvent) Option {
	return func(h *Handler) {
		h.subscribers["Metric"] = subscriber
	}
}

func WithTracePipelineSubscriber(subscriber chan<- event.GenericEvent) Option {
	return func(h *Handler) {
		h.subscribers["Trace"] = subscriber
	}
}

func NewHandler(c client.Reader, opts ...Option) *Handler {
	h := &Handler{
		c:           c,
		logger:      logr.New(logf.NullLogSink{}),
		subscribers: make(map[string]chan<- event.GenericEvent),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

type Alert struct {
	Labels map[string]string `json:"labels"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Info("Invalid method", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	alertsYAML, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error(err, "Failed to read request body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var alerts []Alert
	if unmarshallErr := json.Unmarshal(alertsYAML, &alerts); unmarshallErr != nil {
		h.logger.Error(err, "Failed to unmarshal request body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	h.logger.V(1).Info("Webhook called. Notifying the subscribers.", "request", alerts)

	events := h.metricPipelineReconcileEvents(alerts)
	for _, ev := range events {
		h.subscribers["Metric"] <- ev
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) metricPipelineReconcileEvents(alerts []Alert) []event.GenericEvent {
	var events []event.GenericEvent

	var allPipelines telemetryv1alpha1.MetricPipelineList
	if err := h.c.List(context.TODO(), &allPipelines); err != nil {
		h.logger.Error(err, "Failed to list MetricPipelines")
		return nil
	}

	if slices.ContainsFunc(alerts, func(alert Alert) bool {
		if !strings.HasPrefix(alert.Labels["alertname"], "Metric") {
			return false
		}
		if _, ok := alert.Labels["exporter"]; !ok {
			return true
		}
		return false
	}) {
		for _, mp := range allPipelines.Items {
			events = append(events, event.GenericEvent{Object: &mp})
		}
		return events
	}

	pipelinesToReconcile := slices.DeleteFunc(allPipelines.Items, func(mp telemetryv1alpha1.MetricPipeline) bool {
		return matchesNoAlert(&mp, alerts)
	})

	for _, mp := range pipelinesToReconcile {
		events = append(events, event.GenericEvent{Object: &mp})
	}

	return events
}

func matchesNoAlert(mp *telemetryv1alpha1.MetricPipeline, alerts []Alert) bool {
	for _, alert := range alerts {
		if strings.HasPrefix(alert.Labels["alertname"], "Metric") && matchesPipeline(alert.Labels, mp.Name) {
			return false
		}
	}

	return true
}

func matchesPipeline(labels map[string]string, pipelineName string) bool {
	exportedID, ok := labels["exporter"]
	if !ok {
		return false
	}

	return otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolHTTP, pipelineName) == exportedID || otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolGRPC, pipelineName) == exportedID
}
