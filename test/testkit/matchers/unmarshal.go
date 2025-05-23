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

// UnmarshalPdata reads and unmarshals pdata signals from a JSONL-encoded byte slice (every line is a JSON document encoding pdata).
// It processes each line of the input data and applies the provided unmarshal function.
func UnmarshalPdata[T plog.Logs | pmetric.Metrics | ptrace.Traces](data []byte, unmarshal func(buf []byte) (T, error)) ([]T, error) {
	var allSignals []T

	// User bufio.Reader instead of bufio.Scanner to handle very long lines gracefully
	reader := bufio.NewReader(bytes.NewReader(data))

	for {
		line, readerErr := reader.ReadBytes('\n')
		if readerErr != nil && readerErr != io.EOF {
			return nil, fmt.Errorf("failed to read line: %w", readerErr)
		}

		if len(line) > 0 {
			signals, err := unmarshal(line)
			if err != nil {
				return nil, handleUnmarshalError(err, line, len(data))
			}

			allSignals = append(allSignals, signals)
		}

		// check the io.EOF error after checking the line since both can be returned simultaneously
		if readerErr == io.EOF {
			break
		}
	}

	return allSignals, nil
}

func handleUnmarshalError(err error, line []byte, dataSize int) error {
	size := len(line)

	const maxPreviewSize = 100

	lastElems := line
	if size > maxPreviewSize {
		lastElems = line[size-maxPreviewSize:]
	}

	return fmt.Errorf("failed to unmarshal logs: %w, body size: %d, last 100 elems: %q", err, dataSize, string(lastElems))
}
