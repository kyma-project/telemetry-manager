package stdloggen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendLogLine(t *testing.T) {
	// The line to append
	line := "new log line"

	// Prepare a PodSpec with the default script
	spec := PodSpec(AppendLogLine(line))

	// The script should now contain the new line before "foo bar"
	gotScript := spec.Containers[0].Command[2]

	expectedSrcipt := `while true
do
echo 'new log line'
echo 'foo bar'
sleep 10
done`
	assert.Equal(t, expectedSrcipt, gotScript, "The script should contain the new log line before the first line")
}
