package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/alertrules"
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

type PipelineList interface {
	GetItems() []Pipeline
}

type Pipeline interface {
	GetName() string
}

func (h *Handler) toMetricPipelineReconcileEvents(ctx context.Context, alerts []Alert) []event.GenericEvent {
	return h.toPipelineReconcileEvents(ctx, alerts, &telemetryv1alpha1.MetricPipelineList{}, alertrules.MatchesMetricPipelineRule)
}

func (h *Handler) toTracePipelineReconcileEvents(ctx context.Context, alerts []Alert) []event.GenericEvent {
	return h.toPipelineReconcileEvents(ctx, alerts, &telemetryv1alpha1.TracePipelineList{}, alertrules.MatchesTracePipelineRule)
}

func (h *Handler) toLogPipelineReconcileEvents(ctx context.Context, alerts []Alert) []event.GenericEvent {
	return h.toPipelineReconcileEvents(ctx, alerts, &telemetryv1alpha1.LogPipelineList{}, alertrules.MatchesLogPipelineRule)
}

func (h *Handler) toPipelineReconcileEvents(ctx context.Context,
	alerts []Alert,
	pipelineList client.ObjectList,
	matchFunc func(labels map[string]string, rule string, name string) bool) []event.GenericEvent {

	var events []event.GenericEvent

	pipelines, err := h.list(ctx, pipelineList)
	if err != nil {
		h.logger.Error(err, "Failed to list pipelines", "kind", pipelineList.GetObjectKind().GroupVersionKind().Kind)
		return events
	}

	for i := range pipelines {
		for _, alert := range alerts {
			if matchFunc(alert.Labels, alertrules.RulesAny, pipelines[i].GetName()) {
				events = append(events, event.GenericEvent{Object: pipelines[i]})
			}
		}
	}

	return events
}

// list retrieves an object list of type client.ObjectList and unpacks it into a slice of client.Objects.
func (h *Handler) list(ctx context.Context, objs client.ObjectList) ([]client.Object, error) {
	if err := h.c.List(ctx, objs); err != nil {
		return nil, err
	}

	runtimeObjs, err := meta.ExtractList(objs)
	if err != nil {
		return nil, err
	}

	var objects []client.Object
	for _, runtimeObj := range runtimeObjs {
		objects = append(objects, runtimeObj.(client.Object))
	}

	return objects, nil
}

func retrieveNames(events []event.GenericEvent) []string {
	var names []string
	for _, ev := range events {
		names = append(names, ev.Object.GetName())
	}
	return names
}
