package builder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateInputSection(t *testing.T) {
	expected := `[INPUT]
    name tail
    tag  foo

`
	sut := NewInputSectionBuilder()
	sut.AddConfigParam("name", "tail")
	sut.AddConfigParam("tag", "foo")
	actual := sut.Build()

	require.NotEmpty(t, actual)
	require.Equal(t, expected, actual)
}

func TestCreateOutputSection(t *testing.T) {
	expected := `[OUTPUT]

`
	sut := NewOutputSectionBuilder()
	actual := sut.Build()

	require.NotEmpty(t, actual)
	require.Equal(t, expected, actual)
}

func TestCreateFilterSection(t *testing.T) {
	expected := `[FILTER]

`
	sut := NewFilterSectionBuilder()
	actual := sut.Build()

	require.NotEmpty(t, actual)
	require.Equal(t, expected, actual)
}

func TestCreateSectionWithParams(t *testing.T) {
	expected := `[FILTER]
    key1        value1
    key1        value2
    key2        value2
    longer-key1 value1

`
	sut := NewFilterSectionBuilder()
	sut.AddConfigParam("key2", "value2")
	sut.AddConfigParam("key1", "value2")
	sut.AddConfigParam("key1", "value1")
	sut.AddConfigParam("longer-key1", "value1")
	actual := sut.Build()

	require.NotEmpty(t, actual)
	require.Equal(t, expected, actual)
}
