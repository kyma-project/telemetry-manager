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

// UnmarshalOTLPFile reads and unmarshals pdata signals from a JSONL-encoded byte slice (every line is a JSON document encoding pdata).
// It processes each line of the input data and applies the provided unmarshal function.
func UnmarshalOTLPFile[T plog.Logs | pmetric.Metrics | ptrace.Traces](rawData []byte, unmarshal func(buf []byte) (T, error)) ([]T, error) {
	var otlpData []T

	// User bufio.Reader instead of bufio.Scanner to handle very long lines gracefully
	// Increase default buffer size to 1MB to accommodate larger lines
	const bufSize = 1024 * 1024
	reader := bufio.NewReaderSize(bytes.NewReader(rawData), bufSize)

	for {
		line, readerErr := reader.ReadBytes('\n')
		if readerErr != nil && readerErr != io.EOF {
			return nil, fmt.Errorf("failed to read line: %w", readerErr)
		}

		if len(line) > 0 {
			linePdata, unmarshalErr := unmarshal(line)
			if unmarshalErr != nil {
				if lastLine := readerErr == io.EOF; lastLine {
					// If the error is due to the last line not being complete, we can still return the data read so far
					// but we should not return an error.
					continue
				}

				return nil, handleUnmarshalError(unmarshalErr, line, len(rawData))
			}

			otlpData = append(otlpData, linePdata)
		}

		// check the io.EOF error after checking the line since both can be returned simultaneously
		if readerErr == io.EOF {
			break
		}
	}

	return otlpData, nil
}

func handleUnmarshalError(err error, line []byte, totalLength int) error {
	lineLength := len(line)

	const maxPreviewLength = 100

	lastElems := line
	if lineLength > maxPreviewLength {
		lastElems = line[lineLength-maxPreviewLength:]
	}

	return fmt.Errorf("failed to unmarshal data: total bytes: %d, line bytes: %d, last %d elems: %q, error: %w",
		totalLength,
		lineLength,
		maxPreviewLength,
		string(lastElems),
		err,
	)
}
