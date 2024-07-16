package errortypes

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestError(t *testing.T) {
	err := APIRequestFailedError{
		Err: errors.New("test error"),
	}
	require.Equal(t, "test error", err.Error())
}
