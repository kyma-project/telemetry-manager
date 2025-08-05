package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"math/rand"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"golang.org/x/time/rate"
)

type logFormat string

const (
	defaultByteSize           = 1 << 11 // 2 KiB = 2 * 2^10 = 2^11
	jsonFormat      logFormat = "json"
	plaintextFormat logFormat = "plaintext"
)

var (
	logsGenerated = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "logs_generated_total",
			Help: "Total number of logs generated",
		},
	)

	allowedFormats = map[logFormat]struct{}{
		jsonFormat:      {},
		plaintextFormat: {},
	}
)

func main() {
	formatFlag := pflag.StringP("format", "", string(jsonFormat), fmt.Sprintf("Log format (%s or %s)", jsonFormat, plaintextFormat))
	bytesFlag := pflag.IntP("bytes", "b", defaultByteSize, "Size of each log in bytes")
	logsPerSecondFlag := pflag.IntP("rate", "r", 1, "Approximately how many logs per second each worker should generate. Zero means no throttling")
	workersFlag := pflag.IntP("workers", "w", 1, "Number of workers (goroutines) to run")
	fieldsFlag := pflag.StringToStringP("fields", "f", map[string]string{},
		fmt.Sprintf(`Custom fields in key=value format (comma-separated or repeated). These fields will be included in each %s log record (e.g. --fields key1=value1,key2=value2 or --fields key1=value1 --fields key2=value2). This flag is only relevant when the format is %s.`, jsonFormat, jsonFormat),
	)
	textFlag := pflag.StringP("text", "t", "",
		fmt.Sprintf(`Custom text to be logged in each %s log. This flag will be ignored if the "bytes" flag is provided in which random text will be logged having the size provieded by the "bytes" flag. This flag is only relevant when the format is %s.`, plaintextFormat, plaintextFormat),
	)

	pflag.Parse()

	// Validate the format flag
	format := logFormat(*formatFlag)
	if _, ok := allowedFormats[format]; !ok {
		log.Fatalf("Invalid format: %s. Allowed values are: %s, %s", format, jsonFormat, plaintextFormat)
	}

	// Register the metric
	prometheus.MustRegister(logsGenerated)

	// Expose /metrics endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())

		//nolint:gosec // ignore missing timeout
		err := http.ListenAndServe(":2112", nil)
		if err != nil {
			log.Printf("Error starting HTTP server: %v\n", err)
		}
	}()

	limitPerSecond := rate.Limit(*logsPerSecondFlag)
	if *logsPerSecondFlag == 0 {
		// No limit if logsPerSec is 0
		limitPerSecond = rate.Inf
	}

	// Start workers
	for range *workersFlag {
		switch format {
		case jsonFormat:
			go generateJSONLogs(*bytesFlag, limitPerSecond, *fieldsFlag)
		case plaintextFormat:
			go generatePlaintextLogs(*bytesFlag, limitPerSecond, *textFlag)
		default:
			log.Fatalf("Unexpected log format: %s", format)
		}
	}

	select {}
}

func generateJSONLogs(logSize int, limitPerSecond rate.Limit, fields map[string]string) {
	limiter := rate.NewLimiter(limitPerSecond, 1)

	logRecord := map[string]string{"padding": ""}
	maps.Copy(logRecord, fields)

	JSONLog, err := json.Marshal(logRecord)
	if err != nil {
		log.Fatalf("Error encoding log record to JSON: %v\n", err)
	}

	length := len(JSONLog)

	// Check if the size of the JSON log is already larger than the target size
	const quotes = 2 // number of quotes around every string in JSON

	overhead := length - quotes // subtract the existing quotes for the current empty string in the padding field

	paddingLen := logSize - overhead - quotes // number of characters generated in the padding should exclude the quotes which will be added for the padding field
	if paddingLen < 0 {
		log.Fatalf("The size of the JSON log with custom fields is larger than the requested log size\n")
	}

	for {
		// Pad with random characters until the JSON log reaches the target size
		logRecord["padding"] = offsetString(paddingLen)

		// Avoid using json.Marshal() here forfaster execution
		err = json.NewEncoder(os.Stdout).Encode(logRecord)
		if err != nil {
			log.Fatalf("Error encoding log record to JSON: %v\n", err)
		}

		logsGenerated.Inc()

		if err := limiter.Wait(context.Background()); err != nil {
			log.Printf("Error waiting for rate limiter: %v\n", err)
		}
	}
}

func generatePlaintextLogs(logSize int, limitPerSecond rate.Limit, customText string) string {
	limiter := rate.NewLimiter(limitPerSecond, 1)

	for {
		var plaintextLog string

		if customText != "" {
			// Use the custom text if provided
			plaintextLog = customText
		} else {
			// Generate random text if no custom text is provided
			plaintextLog = offsetString(logSize)
		}

		//nolint:forbidigo // actual printing of the prepared plaintextLog
		fmt.Println(plaintextLog)

		logsGenerated.Inc()

		if err := limiter.Wait(context.Background()); err != nil {
			log.Printf("Error waiting for rate limiter: %v\n", err)
		}
	}
}

func offsetString(length int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// Random starting offset
	// Avoid getting a random character in each loop iteration for faster execution
	//nolint:gosec //no need for cryptographic security here
	start := rand.Intn(len(letters))

	b := make([]byte, length)
	for i := range length {
		pos := (start + start*i) % len(letters)
		b[i] = letters[pos]
	}

	return string(b)
}
