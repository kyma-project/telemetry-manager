package errortypes

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIRequestFailed_Error(t *testing.T) {
	err := APIRequestFailed{
		Err: errors.New("test error"),
	}
	require.Equal(t, "test error", err.Error())
}
