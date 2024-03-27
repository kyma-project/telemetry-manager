package webhook

import (
	"io"
	"net/http"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Handler struct {
	subscribers []chan<- event.GenericEvent
	logger      logr.Logger
}

type Option = func(*Handler)

func WithLogger(logger logr.Logger) Option {
	return func(h *Handler) {
		h.logger = logger
	}
}

func WithSubscriber(subscriber chan<- event.GenericEvent) Option {
	return func(h *Handler) {
		h.subscribers = append(h.subscribers, subscriber)
	}
}

func NewHandler(opts ...Option) *Handler {
	h := &Handler{
		logger: logr.New(logf.NullLogSink{}),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Info("Invalid method", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	req, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error(err, "Failed to read request body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer r.Body.Close()

	h.logger.V(1).Info("Webhook called. Notifying the subscribers.",
		"request", string(req))

	for _, sub := range h.subscribers {
		sub <- event.GenericEvent{}
	}

	w.WriteHeader(http.StatusOK)
}
