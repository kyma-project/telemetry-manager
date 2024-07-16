package errortypes

import (
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAPIRequestFailed_Error(t *testing.T) {
	err := APIRequestFailed{
		Err: errors.New("test error"),
	}
	require.Equal(t, "test error", err.Error())
}
