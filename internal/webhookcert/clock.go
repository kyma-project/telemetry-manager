package webhookcert

import (
	"time"
)

type clock interface {
	now() time.Time
}

type realClock struct {
}

func (realClock) now() time.Time {
	return time.Now().UTC()
}
