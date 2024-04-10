package webhook

import (
	"context"
	"encoding/json"
	"github.com/go-logr/logr"
	"io"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/alertrules"
)

type Handler struct {
	c           client.Reader
	subscribers map[string]chan<- event.GenericEvent
	logger      logr.Logger
}

type Option = func(*Handler)

const (
	metricPipelineSubscriber = "metricPipelines"
	tracePipelineSubscriber  = "tracePipelines"
	logPipelineSubscriber    = "logPipelines"
)

func WithMetricPipelineSubscriber(subscriber chan<- event.GenericEvent) Option {
	return withSubscriber(subscriber, metricPipelineSubscriber)
}

func WithTracePipelineSubscriber(subscriber chan<- event.GenericEvent) Option {
	return withSubscriber(subscriber, tracePipelineSubscriber)
}

func WithLogPipelineSubscriber(subscriber chan<- event.GenericEvent) Option {
	return withSubscriber(subscriber, logPipelineSubscriber)
}

func withSubscriber(subscriber chan<- event.GenericEvent, pipelineType string) Option {
	return func(h *Handler) {
		h.subscribers[pipelineType] = subscriber
	}
}

func WithLogger(logger logr.Logger) Option {
	return func(h *Handler) {
		h.logger = logger
	}
}

// NewHandler creates a new self-monitor webhook handler.
// This handler serves an endpoint that mimics Alertmanager, allowing Prometheus to send alerts to it.
// The handler then notifies the subscribers, typically controllers, about the alerts that match the pipelines.
// Subsequently, the subscribers reconcile the pipelines based on the received alerts.
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
	logPipelineEvents := h.toLogPipelineReconcileEvents(r.Context(), alerts)
	h.logger.V(1).Info("Webhook called. Notifying the subscribers.",
		"request", alerts,
		"metricPipelines", retrieveNames(metricPipelineEvents),
		"tracePipelines", retrieveNames(tracePipelineEvents),
		"logPipelines", retrieveNames(logPipelineEvents),
	)

	for _, ev := range metricPipelineEvents {
		h.subscribers[metricPipelineSubscriber] <- ev
	}

	for _, ev := range tracePipelineEvents {
		h.subscribers[tracePipelineSubscriber] <- ev
	}

	for _, ev := range logPipelineEvents {
		h.subscribers[logPipelineSubscriber] <- ev
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
		pipelineName := metricPipelines.Items[i].GetName()
		for _, alert := range alerts {
			if alertrules.MatchesMetricPipelineRule(alert.Labels, alertrules.RulesAny, pipelineName) {
				events = append(events, event.GenericEvent{Object: &metricPipelines.Items[i]})
			}
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
		pipelineName := tracePipelines.Items[i].GetName()
		for _, alert := range alerts {
			if alertrules.MatchesTracePipelineRule(alert.Labels, alertrules.RulesAny, pipelineName) {
				events = append(events, event.GenericEvent{Object: &tracePipelines.Items[i]})
			}
		}
	}

	return events
}

func (h *Handler) toLogPipelineReconcileEvents(ctx context.Context, alerts []Alert) []event.GenericEvent {
	var events []event.GenericEvent
	var logPipelines telemetryv1alpha1.LogPipelineList
	if err := h.c.List(ctx, &logPipelines); err != nil {
		return events
	}

	for i := range logPipelines.Items {
		pipelineName := logPipelines.Items[i].GetName()
		for _, alert := range alerts {
			if alertrules.MatchesLogPipelineRule(alert.Labels, alertrules.RulesAny, pipelineName) {
				events = append(events, event.GenericEvent{Object: &logPipelines.Items[i]})
			}
		}
	}

	return events
}

func retrieveNames(events []event.GenericEvent) []string {
	var names []string
	for _, ev := range events {
		names = append(names, ev.Object.GetName())
	}
	return names
}
