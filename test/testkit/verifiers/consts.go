package verifiers

import (
	"time"
)

const (
	timeout               = time.Second * 60
	interval              = time.Millisecond * 250
	reconciliationTimeout = time.Second * 10
)
