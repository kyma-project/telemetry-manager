package webhookcert

import (
	"time"
)

type mockClock struct {
	t time.Time
}

func (c mockClock) now() time.Time {
	return c.t
}
