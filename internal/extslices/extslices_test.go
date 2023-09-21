package extslices

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type intSlice []int

func TestTransformFunc(t *testing.T) {
	tests := []struct {
		name          string
		input         []int
		transformFunc func(int) string
		expected      []string
	}{
		{
			name:  "should transform",
			input: []int{1, 2, 3},
			transformFunc: func(e int) string {
				return strconv.Itoa(e)
			},
			expected: []string{"1", "2", "3"},
		},
		{
			name:  "should transform with type alias",
			input: intSlice{1, 2, 3},
			transformFunc: func(e int) string {
				return strconv.Itoa(e)
			},
			expected: []string{"1", "2", "3"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			transformed := TransformFunc(tc.input, tc.transformFunc)
			require.Equal(t, tc.expected, transformed, tc.name)
		})
	}
}
