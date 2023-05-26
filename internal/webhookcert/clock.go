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

type mockClock struct {
	t time.Time
}

func (c mockClock) now() time.Time {
	return c.t
}
