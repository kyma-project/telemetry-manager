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
	var allPdata []T

	// User bufio.Reader instead of bufio.Scanner to handle very long lines gracefully
	// Increase default buffer size to 64KB to accommodate larger lines
	const bufSize = 64 * 1024
	reader := bufio.NewReaderSize(bytes.NewReader(data), bufSize)

	for {
		line, readerErr := reader.ReadBytes('\n')
		if readerErr != nil && readerErr != io.EOF {
			return nil, fmt.Errorf("failed to read line: %w", readerErr)
		}

		if len(line) > 0 {
			linePdata, err := unmarshal(line)
			if err != nil {
				return nil, handleUnmarshalError(err, line, len(data))
			}

			allPdata = append(allPdata, linePdata)
		}

		// check the io.EOF error after checking the line since both can be returned simultaneously
		if readerErr == io.EOF {
			break
		}
	}

	return allPdata, nil
}

func handleUnmarshalError(err error, line []byte, totalLength int) error {
	lineLength := len(line)

	const maxPreviewLength = 100

	lastElems := line
	if lineLength > maxPreviewLength {
		lastElems = line[lineLength-maxPreviewLength:]
	}

	return fmt.Errorf("failed to unmarshal pdata: total bytes: %d, line bytes: %d, last %d elems: %q, error: %w",
		totalLength,
		lineLength,
		maxPreviewLength,
		string(lastElems),
		err,
	)
}
