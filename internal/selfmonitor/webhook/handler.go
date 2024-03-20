package webhook

import (
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type Handler struct {
	eventChan chan<- event.GenericEvent
}

func NewHandler(eventChan chan<- event.GenericEvent) *Handler {
	return &Handler{
		eventChan: eventChan,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.eventChan <- event.GenericEvent{}

	w.WriteHeader(http.StatusOK)
}
