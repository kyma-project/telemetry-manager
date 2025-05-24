package stdloggen

import (
	"fmt"
	"testing"
)

func TestAppendLogLine(t *testing.T) {
	// The line to append
	line := "new log line"

	// Prepare a PodSpec with the default script
	spec := PodSpec(AppendLogLine(line))

	// The script should now contain the new line before "foo bar"
	gotScript := spec.Containers[0].Command[2]

	fmt.Printf("Got script: %s\n", gotScript)
}
