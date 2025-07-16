package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
)

var logsGenerated = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "logs_generated_total",
		Help: "Total number of logs generated",
	},
)

func main() {
	bytesFlag := pflag.IntP("bytes", "b", 2048, "Size of each log in bytes")
	rateFlag := pflag.IntP("rate", "r", 1, "Approximately how many logs per second each worker should generate")
	workersFlag := pflag.IntP("workers", "w", 1, "Number of workers (goroutines) to run")
	fieldsFlag := pflag.StringToStringP("fields", "f", map[string]string{}, "Custom fields in key=value format (comma-separated or repeated). These fields will be included in each log record. (e.g. --fields key1=value1,key2=value2 or --fields key1=value1 --fields key2=value2)")

	pflag.Parse()

	// Register the metric
	prometheus.MustRegister(logsGenerated)

	// Expose /metrics endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":2112", nil)
	}()

	// Start workers
	for i := 0; i < *workersFlag; i++ {
		go generateLogs(*bytesFlag, *rateFlag, *fieldsFlag)
	}

	select {}
}

func generateLogs(logSize int, rate int, fields map[string]string) {
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	defer ticker.Stop()

	for range ticker.C {
		// Create a log record with the provided fields
		logRecord := make(map[string]string)
		maps.Copy(logRecord, fields)

		// Add a timestamp and initially an empty body to the log record
		logRecord["timestamp"] = time.Now().Format(time.RFC3339)
		body := ""
		logRecord["body"] = body
		logJson, err := json.Marshal(logRecord)
		if err != nil {
			fmt.Printf("Error marshaling log record: %v\n", err)
			os.Exit(1)
		}

		// Check if the JSON log is already larger than the target size
		if len(logJson) > logSize {
			fmt.Printf("The log record with just the timestamp is already larger than the target size of %d bytes.\n", logSize)
			os.Exit(1)
		}

		// Pad the body until the JSON log reaches the target size
		for len(logJson) < logSize {
			body += "a"
			logRecord["body"] = body
			logJson, err = json.Marshal(logRecord)
			if err != nil {
				fmt.Printf("Error marshaling log record: %v\n", err)
				os.Exit(1)
			}
		}

		fmt.Println(string(logJson))

		// Increment the counter for each log generated
		logsGenerated.Inc()
	}
}
