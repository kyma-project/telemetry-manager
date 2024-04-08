package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	"github.com/prometheus/common/model"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/alertrules"
)

type Handler struct {
	c           client.Reader
	subscribers map[alertrules.PipelineType]chan<- event.GenericEvent
	logger      logr.Logger
}

type Option = func(*Handler)

func WithSubscriber(subscriber chan<- event.GenericEvent, pipelineType alertrules.PipelineType) Option {
	return func(h *Handler) {
		h.subscribers[pipelineType] = subscriber
	}
}

func WithLogger(logger logr.Logger) Option {
	return func(h *Handler) {
		h.logger = logger
	}
}

func NewHandler(c client.Reader, opts ...Option) *Handler {
	h := &Handler{
		c:           c,
		logger:      logr.New(logf.NullLogSink{}),
		subscribers: make(map[alertrules.PipelineType]chan<- event.GenericEvent),
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
	w.Header().Set("Content-Security-Policy", "default-src 'self'")

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

	metricPipelineEvents := h.toMetricPipelineReconcileEvents(r.Context(), alerts)
	tracePipelineEvents := h.toTracePipelineReconcileEvents(r.Context(), alerts)
	h.logger.V(1).Info("Webhook called. Notifying the subscribers.",
		"request", alerts,
		"metricPipelines", retrieveNames(metricPipelineEvents),
		"tracePipelines", retrieveNames(tracePipelineEvents),
	)

	for _, ev := range metricPipelineEvents {
		h.subscribers[alertrules.MetricPipeline] <- ev
	}

	for _, ev := range tracePipelineEvents {
		h.subscribers[alertrules.TracePipeline] <- ev
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) toMetricPipelineReconcileEvents(ctx context.Context, alerts []Alert) []event.GenericEvent {
	var events []event.GenericEvent
	var metricPipelines telemetryv1alpha1.MetricPipelineList
	if err := h.c.List(ctx, &metricPipelines); err != nil {
		return events
	}

	for i := range metricPipelines.Items {
		if shouldReconcile(&metricPipelines.Items[i], alertrules.MetricPipeline, alerts) {
			events = append(events, event.GenericEvent{Object: &metricPipelines.Items[i]})
		}
	}

	return events
}

func (h *Handler) toTracePipelineReconcileEvents(ctx context.Context, alerts []Alert) []event.GenericEvent {
	var events []event.GenericEvent
	var tracePipelines telemetryv1alpha1.TracePipelineList
	if err := h.c.List(ctx, &tracePipelines); err != nil {
		return events
	}

	for i := range tracePipelines.Items {
		if shouldReconcile(&tracePipelines.Items[i], alertrules.TracePipeline, alerts) {
			events = append(events, event.GenericEvent{Object: &tracePipelines.Items[i]})
		}
	}

	return events
}

func shouldReconcile(pipeline client.Object, pipelineType alertrules.PipelineType, alerts []Alert) bool {
	for _, alert := range alerts {
		expectedPrefix := alertrules.RuleNamePrefix(pipelineType)
		if !strings.HasPrefix(alert.Labels[model.AlertNameLabel], expectedPrefix) {
			continue
		}

		if matchesAllPipelines(alert.Labels) || matchesPipeline(alert.Labels, pipeline.GetName()) {
			return true
		}
	}

	return false
}

func matchesAllPipelines(labels map[string]string) bool {
	if _, ok := labels[alertrules.LabelExporter]; !ok {
		return true
	}
	return false
}

func matchesPipeline(labels map[string]string, pipelineName string) bool {
	exportedID, ok := labels[alertrules.LabelExporter]
	if !ok {
		return false
	}

	return otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolHTTP, pipelineName) == exportedID || otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolGRPC, pipelineName) == exportedID
}

func retrieveNames(events []event.GenericEvent) []string {
	var names []string
	for _, ev := range events {
		names = append(names, ev.Object.GetName())
	}
	return names
}
