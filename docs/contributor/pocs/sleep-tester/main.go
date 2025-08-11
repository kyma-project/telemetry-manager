package main

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
)

func main() {
	sleepDuration := pflag.DurationP("duration", "d", 1*time.Millisecond, "Duration to sleep")
	pflag.Parse()

	const totalAttempts = 100
	var totalSleepDuration time.Duration

	for range totalAttempts {
		startTime := time.Now()
		time.Sleep(*sleepDuration)
		actualSleepDuration := time.Since(startTime)

		// Accumulate the actual sleep durations to calculate the average at the end
		totalSleepDuration += actualSleepDuration
		fmt.Printf("Slept: %v\n", actualSleepDuration)
	}

	fmt.Printf("\nAverage sleep time: %v\n", totalSleepDuration/totalAttempts)
}
