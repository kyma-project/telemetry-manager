package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmptyFlagValueReturnsError(t *testing.T) {
	var m CLIMapFlag

	err := m.Set("")
	require.Error(t, err)
	require.Equal(t, "empty flag value", err.Error())
}

func TestValidFlagParsesCorrectly(t *testing.T) {
	var m CLIMapFlag

	err := m.Set("key1=value1,key2=value2")
	require.NoError(t, err)
	require.Equal(t, "value1", m["key1"])
	require.Equal(t, "value2", m["key2"])
}

func TestInvalidEntryWithoutEqualsReturnsError(t *testing.T) {
	var m CLIMapFlag

	err := m.Set("key1=value1,key2")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid entry")
}

func TestEmptyKeyReturnsError(t *testing.T) {
	var m CLIMapFlag

	err := m.Set("=value1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty key")
}

func TestEmptyValueIsAllowed(t *testing.T) {
	var m CLIMapFlag

	err := m.Set("key1=")
	require.NoError(t, err)
	require.Equal(t, "", m["key1"])
}

func TestWhitespaceAroundEntriesIsTrimmed(t *testing.T) {
	var m CLIMapFlag

	err := m.Set(" key1 = value1 , key2 = value2 ")
	require.NoError(t, err)
	require.Equal(t, "value1", m["key1"])
	require.Equal(t, "value2", m["key2"])
}

func TestNilMapIsInitialized(t *testing.T) {
	var m CLIMapFlag

	err := m.Set("key1=value1")
	require.NoError(t, err)
	require.Equal(t, "value1", m["key1"])
}

func TestCLIMapFlagStringReturnsContents(t *testing.T) {
	m := CLIMapFlag{"a": "1", "b": "2"}
	s := m.String()
	require.True(t, strings.Contains(s, "a:1"))
	require.True(t, strings.Contains(s, "b:2"))
}

func TestSetIgnoresEmptyEntries(t *testing.T) {
	var m CLIMapFlag

	err := m.Set("key1=value1,,key2=value2,")
	require.NoError(t, err)
	require.Equal(t, "value1", m["key1"])
	require.Equal(t, "value2", m["key2"])
}

func TestSetReinitializesMap(t *testing.T) {
	m := CLIMapFlag{"old": "x"}
	err := m.Set("new=1")
	require.NoError(t, err)
	require.Equal(t, "1", m["new"])
	require.Nil(t, func() any {
		_, ok := m["old"]
		if ok {
			return "exists"
		}

		return nil
	}())
}
