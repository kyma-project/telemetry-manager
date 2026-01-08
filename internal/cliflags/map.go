package cliflags

import (
	"fmt"
	"strings"
)

type Map map[string]string

func (m *Map) String() string {
	return fmt.Sprintf("%v", *m)
}

func (m *Map) Set(s string) error {
	const sliceCount = 2

	parts := strings.SplitN(s, "=", sliceCount)
	if len(parts) != sliceCount {
		return fmt.Errorf("invalid format %q, expected key=value", s)
	}

	key := parts[0]
	val := parts[1]

	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("empty key in %q", s)
	}

	if *m == nil {
		*m = make(map[string]string)
	}

	(*m)[strings.TrimSpace(key)] = strings.TrimSpace(val)

	return nil
}
