package config

import (
	"fmt"
	"strings"
)

type CLIMapFlag map[string]string

func (m *CLIMapFlag) String() string {
	return fmt.Sprintf("%v", *m)
}

func (m *CLIMapFlag) Set(s string) error {
	*m = make(CLIMapFlag)

	if s == "" {
		return fmt.Errorf("empty flag value")
	}

	entries := strings.SplitSeq(s, ",")
	for e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}

		sliceCount := 2

		parts := strings.SplitN(e, "=", sliceCount)
		if len(parts) != sliceCount {
			return fmt.Errorf("invalid entry %q, expected key=value", e)
		}

		key := strings.TrimSpace(parts[0])
		if key == "" {
			return fmt.Errorf("empty key in %q", e)
		}

		value := strings.TrimSpace(parts[1])
		(*m)[key] = value
	}

	return nil
}
