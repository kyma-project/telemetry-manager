package pipelines

import "fmt"

type SignalType string

const (
	SignalTypeTrace        SignalType = "trace"
	SignalTypeMetric       SignalType = "metric"
	SignalTypeLog          SignalType = "log"
	SignalTypeLogFluentBit SignalType = "log-fluentbit"
)

func (s SignalType) Validate() error {
	if s == SignalTypeTrace || s == SignalTypeMetric || s == SignalTypeLog || s == SignalTypeLogFluentBit {
		return nil
	}

	return fmt.Errorf("invalid SignalType: %s", s)
}
