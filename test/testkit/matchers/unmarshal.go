package matchers

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func UnmarshalSignals[T plog.Logs | pmetric.Metrics | ptrace.Traces](jsonlSignals []byte, unmarshal func(buf []byte) (T, error)) ([]T, error) {
	var allSignals []T

	// User bufio.Reader instead of bufio.Scanner to handle very long lines gracefully
	reader := bufio.NewReader(bytes.NewReader(jsonlSignals))
	for {
		line, readerErr := reader.ReadBytes('\n')
		if readerErr != nil && readerErr != io.EOF {
			return nil, fmt.Errorf("failed to read line: %v", readerErr)
		}

		if len(line) > 0 {
			signals, err := unmarshal(line)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal logs: %v", readerErr)
			}
			allSignals = append(allSignals, signals)
		}

		//check the io.EOF error after checking the line since both can be returned simultaneously
		if readerErr == io.EOF {
			break
		}
	}

	return allSignals, nil
}
