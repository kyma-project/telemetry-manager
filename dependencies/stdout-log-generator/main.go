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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"golang.org/x/time/rate"
)

type logFormat string

const (
	defaultByteSize                     = 1 << 11 // 2 KiB = 2 * 2^10 = 2^11
	jsonFormat                logFormat = "json"
	plaintextFormat           logFormat = "plaintext"
	millisecondsPerSecond               = 1000
	twoMillisecondsMultiplier           = 2
)

var (
	logsGeneratedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "logs_generated_total",
			Help: "Total number of logs generated",
		},
	)

	logsGeneratedRate = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "logs_generated_rate",
			Help: "Actual rate of logs generated per second",
		},
	)

	allowedFormats = map[logFormat]struct{}{
		jsonFormat:      {},
		plaintextFormat: {},
	}
)

func main() {
	startTime := time.Now()

	formatFlag := pflag.StringP("format", "", string(jsonFormat), fmt.Sprintf("Log format (%s or %s)", jsonFormat, plaintextFormat))
	bytesFlag := pflag.IntP("bytes", "b", defaultByteSize, "Size of each log in bytes")
	logsPerSecondFlag := pflag.IntP("rate", "r", 1, "Approximately how many logs per second each worker should generate. Zero means no throttling")
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

	// Register the metrics
	prometheus.MustRegister(logsGeneratedTotal, logsGeneratedRate)

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

	// On linux, the minimum possible sleep time is around 2ms (this is practically proven in the POC ../../docs/contributor/pocs/sleep-tester/sleep-tester.md)
	// So, to achieve for example a rate of around 70,000 logs per second (70 logs every 1ms), we need to have 140 logs every 2ms
	// And since the timer will only be able to tick every 2ms, the burst size needs to be 140
	// This will allow 140 logs back-to-back followed by around 2ms sleep, then another 140 logs back-to-back followed by around 2ms, etc.
	// For more details, check the issue https://github.com/golang/go/issues/47084#issuecomment-897291261
	limitPerMillisecond := int(limitPerSecond / millisecondsPerSecond)
	limiter := rate.NewLimiter(limitPerSecond, max(1, twoMillisecondsMultiplier*limitPerMillisecond))

	// Start generation of logs
	switch format {
	case jsonFormat:
		generateJSONLogs(startTime, *bytesFlag, limiter, *fieldsFlag)
	case plaintextFormat:
		generatePlaintextLogs(startTime, *bytesFlag, limiter, *textFlag)
	default:
		log.Fatalf("Unexpected log format: %s", format)
	}
}

func generateJSONLogs(startTime time.Time, logSize int, limiter *rate.Limiter, fields map[string]string) {
	logsCounter := 0

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

		// Avoid using json.Marshal() here for faster execution
		err = json.NewEncoder(os.Stdout).Encode(logRecord)
		if err != nil {
			log.Fatalf("Error encoding log record to JSON: %v\n", err)
		}

		logsCounter++

		logsGeneratedTotal.Inc()
		logsGeneratedRate.Set(float64(logsCounter) / time.Since(startTime).Seconds())

		if err := limiter.Wait(context.Background()); err != nil {
			log.Printf("Error waiting for rate limiter: %v\n", err)
		}
	}
}

func generatePlaintextLogs(startTime time.Time, logSize int, limiter *rate.Limiter, customText string) string {
	logsCounter := 0

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

		logsCounter++

		logsGeneratedTotal.Inc()
		logsGeneratedRate.Set(float64(logsCounter) / time.Since(startTime).Seconds())

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
