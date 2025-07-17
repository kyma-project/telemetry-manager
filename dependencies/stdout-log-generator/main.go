package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"golang.org/x/time/rate"
)

var logsGenerated = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "logs_generated_total",
		Help: "Total number of logs generated",
	},
)

func main() {
	bytes := pflag.IntP("bytes", "b", 2048, "Size of each log in bytes")
	logsPerSecond := pflag.IntP("rate", "r", 1, "Approximately how many logs per second each worker should generate. Zero means no throttling")
	workers := pflag.IntP("workers", "w", 1, "Number of workers (goroutines) to run")
	fields := pflag.StringToStringP("fields", "f", map[string]string{}, "Custom fields in key=value format (comma-separated or repeated). These fields will be included in each log record (e.g. --fields key1=value1,key2=value2 or --fields key1=value1 --fields key2=value2)")

	pflag.Parse()

	// Register the metric
	prometheus.MustRegister(logsGenerated)

	// Expose /metrics endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":2112", nil)
	}()

	limitPerSecond := rate.Limit(*logsPerSecond)
	if *logsPerSecond == 0 {
		// No limit if logsPerSec is 0
		limitPerSecond = rate.Inf
	}

	// Start workers
	for i := 0; i < *workers; i++ {
		go generateLogs(*bytes, limitPerSecond, *fields)
	}

	select {}
}

func generateLogs(logSize int, limitPerSecond rate.Limit, fields map[string]string) {
	limiter := rate.NewLimiter(limitPerSecond, 1)

	for {
		// Create a log record with the provided fields
		logRecord := make(map[string]string)
		maps.Copy(logRecord, fields)

		// Add a timestamp and initially an empty body to the log record
		logRecord["timestamp"] = time.Now().Format(time.RFC3339)
		logRecord["body"] = ""
		logJson, err := json.Marshal(logRecord)
		if err != nil {
			fmt.Printf("Error marshaling log record: %v\n", err)
			os.Exit(1)
		}

		// Check if the size of the JSON log is already larger than the target size
		overhead := len(logJson) - len(`""`) // subtract the empty body quotes in logJson
		bodyLen := logSize - overhead - 2    // subtract 2 for the quotes around the body value
		if bodyLen < 0 {
			fmt.Printf("The size of the JSON log with custom fields and a timestamp is larger than the requested log size\n")
			os.Exit(1)
		}

		// Pad the body until the JSON log reaches the target size
		logRecord["body"] = strings.Repeat("a", bodyLen)
		logJson, err = json.Marshal(logRecord)
		if err != nil {
			fmt.Printf("Error marshaling log record: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(string(logJson))

		logsGenerated.Inc()

		if err := limiter.Wait(context.Background()); err != nil {
			fmt.Printf("Error waiting for rate limiter: %v\n", err)
		}
	}
}
