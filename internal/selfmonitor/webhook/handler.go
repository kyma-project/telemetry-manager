package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	selfmonitorconfig "github.com/kyma-project/telemetry-manager/internal/selfmonitor/config"
)

type Handler struct {
	c           client.Reader
	subscribers map[subscriberType]chan<- event.GenericEvent
	logger      logr.Logger
}

type Option = func(*Handler)

type subscriberType int

const (
	subscriberMetricPipeline subscriberType = iota
	subscriberTracePipeline
	subscriberLogPipeline
)

func WithMetricPipelineSubscriber(subscriber chan<- event.GenericEvent) Option {
	return withSubscriber(subscriber, subscriberMetricPipeline)
}

func WithTracePipelineSubscriber(subscriber chan<- event.GenericEvent) Option {
	return withSubscriber(subscriber, subscriberTracePipeline)
}

func WithLogPipelineSubscriber(subscriber chan<- event.GenericEvent) Option {
	return withSubscriber(subscriber, subscriberLogPipeline)
}

func withSubscriber(sub chan<- event.GenericEvent, subType subscriberType) Option {
	return func(h *Handler) {
		h.subscribers[subType] = sub
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
		subscribers: make(map[subscriberType]chan<- event.GenericEvent),
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

	const MB = 1 << 20 // 1 MB

	defer r.Body.Close()

	var alerts []Alert

	decoder := json.NewDecoder(io.LimitReader(r.Body, 1*MB))
	if err := decoder.Decode(&alerts); err != nil {
		h.logger.Error(err, "Failed to unmarshal request body")
		w.WriteHeader(http.StatusBadRequest)

		return
	}

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
		h.subscribers[subscriberMetricPipeline] <- ev
	}

	for _, ev := range tracePipelineEvents {
		h.subscribers[subscriberTracePipeline] <- ev
	}

	for _, ev := range logPipelineEvents {
		h.subscribers[subscriberLogPipeline] <- ev
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) toMetricPipelineReconcileEvents(ctx context.Context, alerts []Alert) []event.GenericEvent { //nolint:dupl // The functions are similar but not identical
	var events []event.GenericEvent

	var metricPipelines telemetryv1beta1.MetricPipelineList
	if err := h.c.List(ctx, &metricPipelines); err != nil {
		return events
	}

	for i := range metricPipelines.Items {
		pipelineName := metricPipelines.Items[i].GetName()
		for _, alert := range alerts {
			if selfmonitorconfig.MatchesMetricPipelineRule(alert.Labels, selfmonitorconfig.RulesAny, pipelineName) {
				events = append(events, event.GenericEvent{Object: &metricPipelines.Items[i]})
			}
		}
	}

	return events
}

func (h *Handler) toTracePipelineReconcileEvents(ctx context.Context, alerts []Alert) []event.GenericEvent { //nolint:dupl // The functions are similar but not identical
	var events []event.GenericEvent

	var tracePipelines telemetryv1beta1.TracePipelineList
	if err := h.c.List(ctx, &tracePipelines); err != nil {
		return events
	}

	for i := range tracePipelines.Items {
		pipelineName := tracePipelines.Items[i].GetName()
		for _, alert := range alerts {
			if selfmonitorconfig.MatchesTracePipelineRule(alert.Labels, selfmonitorconfig.RulesAny, pipelineName) {
				events = append(events, event.GenericEvent{Object: &tracePipelines.Items[i]})
			}
		}
	}

	return events
}

func (h *Handler) toLogPipelineReconcileEvents(ctx context.Context, alerts []Alert) []event.GenericEvent { //nolint:dupl // The functions are similar but not identical
	var events []event.GenericEvent

	var logPipelines telemetryv1beta1.LogPipelineList
	if err := h.c.List(ctx, &logPipelines); err != nil {
		return events
	}

	for i := range logPipelines.Items {
		pipelineName := logPipelines.Items[i].GetName()
		for _, alert := range alerts {
			if selfmonitorconfig.MatchesLogPipelineRule(alert.Labels, selfmonitorconfig.RulesAny, pipelineName) {
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
