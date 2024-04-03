package webhook

import (
	"context"
	"encoding/json"
	"github.com/go-logr/logr"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor"
	"github.com/prometheus/common/model"
	"io"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

type Handler struct {
	c           client.Reader
	subscribers map[selfmonitor.PipelineType]chan<- event.GenericEvent
	logger      logr.Logger
}

type Option = func(*Handler)

func WithSubscriber(subscriber chan<- event.GenericEvent, pipelineType selfmonitor.PipelineType) Option {
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
		subscribers: make(map[selfmonitor.PipelineType]chan<- event.GenericEvent),
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

	ctx := context.TODO()

	for _, ev := range h.toMetricPipelineReconcileEvents(ctx, alerts) {
		h.subscribers[selfmonitor.MetricPipeline] <- ev
	}

	for _, ev := range h.toTracePipelineReconcileEvents(ctx, alerts) {
		h.subscribers[selfmonitor.TracePipeline] <- ev
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) toMetricPipelineReconcileEvents(ctx context.Context, alerts []Alert) []event.GenericEvent {
	var events []event.GenericEvent

	var allPipelines telemetryv1alpha1.MetricPipelineList
	if err := h.c.List(context.TODO(), &allPipelines); err != nil {
		h.logger.Error(err, "Failed to list MetricPipelines")
		return nil
	}

	for i := range allPipelines.Items {
		if shouldReconcile(&allPipelines.Items[i], selfmonitor.MetricPipeline, alerts) {
			events = append(events, event.GenericEvent{Object: &allPipelines.Items[i]})
		}
	}

	return events
}

func (h *Handler) toTracePipelineReconcileEvents(ctx context.Context, alerts []Alert) []event.GenericEvent {
	var events []event.GenericEvent

	var allPipelines telemetryv1alpha1.TracePipelineList
	if err := h.c.List(context.TODO(), &allPipelines); err != nil {
		h.logger.Error(err, "Failed to list TracePipelines")
		return nil
	}

	for i := range allPipelines.Items {
		if shouldReconcile(&allPipelines.Items[i], selfmonitor.TracePipeline, alerts) {
			events = append(events, event.GenericEvent{Object: &allPipelines.Items[i]})
		}
	}

	return events
}

func shouldReconcile(pipeline client.Object, pipelineType selfmonitor.PipelineType, alerts []Alert) bool {
	for _, alert := range alerts {
		if !strings.HasPrefix(alert.Labels[model.AlertNameLabel], string(pipelineType)) {
			continue
		}

		if matchesAllPipelines(alert.Labels) || matchesPipeline(alert.Labels, pipeline.GetName()) {
			return true
		}
	}

	return false
}

func matchesAllPipelines(labels map[string]string) bool {
	if _, ok := labels["exporter"]; !ok {
		return true
	}
	return false
}

func matchesPipeline(labels map[string]string, pipelineName string) bool {
	exportedID, ok := labels["exporter"]
	if !ok {
		return false
	}

	return otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolHTTP, pipelineName) == exportedID || otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolGRPC, pipelineName) == exportedID
}
