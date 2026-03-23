package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	maxPercentage    = 100
	ruleParts        = 2
	statsInterval    = time.Second
	grpcFrameHdrSize = 5 // 1 byte compression flag + 4 bytes message length
)

// gRPC status codes used for fault injection.
// See https://grpc.github.io/grpc/core/md_doc_statuscodes.html
const (
	grpcStatusInvalidArgument   = 3  // non-retryable; maps to HTTP 400
	grpcStatusResourceExhausted = 8  // retryable;     maps to HTTP 429
	grpcStatusInternal          = 13 // non-retryable; maps to HTTP 500
)

// httpToGRPCStatus maps an HTTP fault status code to the gRPC status code that
// produces the same retryability behavior in the OTel Collector OTLP exporter.
//
// OTel Collector retryable gRPC codes: Canceled, DeadlineExceeded, Aborted,
// OutOfRange, Unavailable, DataLoss, ResourceExhausted (with Retry-After).
// Everything else is permanent / non-retryable.
func httpToGRPCStatus(httpCode int) int {
	switch httpCode {
	case http.StatusTooManyRequests: // 429 – retryable for both OTel and Fluent Bit
		return grpcStatusResourceExhausted
	case http.StatusBadRequest: // 400 – non-retryable for both
		return grpcStatusInvalidArgument
	case http.StatusInternalServerError: // 500 – non-retryable for OTel
		return grpcStatusInternal
	default:
		return grpcStatusInternal
	}
}

type rule struct {
	statusCode int
	percentage float64
}

type stats struct {
	totalRequests atomic.Int64
	counters      []statusCounter
}

type statusCounter struct {
	code  int
	count atomic.Int64
}

func (s *stats) record(code int) {
	s.totalRequests.Add(1)

	for i := range s.counters {
		if s.counters[i].code == code {
			s.counters[i].count.Add(1)
			return
		}
	}
}

func (s *stats) logAndReset() {
	total := s.totalRequests.Swap(0)
	if total == 0 {
		return
	}

	var parts []string

	for i := range s.counters {
		count := s.counters[i].count.Swap(0)
		if count > 0 {
			parts = append(parts, fmt.Sprintf("%d→%d", s.counters[i].code, count))
		}
	}

	log.Printf("requests/s: %d (%s)", total, strings.Join(parts, ", "))
}

func newStats(rules []rule, defaultBehavior string) *stats {
	codeSet := make(map[int]bool)
	for _, r := range rules {
		codeSet[r.statusCode] = true
	}

	if defaultBehavior == "close" {
		codeSet[0] = true
	} else {
		code, _ := strconv.Atoi(defaultBehavior) //nolint:errcheck // already validated by parseConfig
		codeSet[code] = true
	}

	s := &stats{
		counters: make([]statusCounter, 0, len(codeSet)),
	}
	for code := range codeSet {
		s.counters = append(s.counters, statusCounter{code: code})
	}

	return s
}

func main() {
	log.SetPrefix(fmt.Sprintf("[%s] ", "mock-backend"))

	rules, defaultBehavior := parseConfig()
	reqStats := newStats(rules, defaultBehavior)

	go func() {
		ticker := time.NewTicker(statsInterval)
		defer ticker.Stop()

		for range ticker.C {
			reqStats.logAndReset()
		}
	}()

	handler := buildHandler(rules, defaultBehavior, reqStats)

	ports := []string{":4317", ":4318", ":9880"}

	var wg sync.WaitGroup
	for _, port := range ports {
		wg.Go(func() {
			var lc net.ListenConfig

			ln, err := lc.Listen(context.Background(), "tcp", port)
			if err != nil {
				log.Fatalf("Failed to listen on %s: %v", port, err)
			}

			log.Printf("Listening on %s", port)

			// Wrap handler with h2c to support HTTP/2 cleartext with prior knowledge,
			// which is how gRPC clients (e.g. OTel Collector OTLP exporter) connect.
			h2cHandler := h2c.NewHandler(handler, &http2.Server{})

			//nolint:gosec // no timeouts needed for test-only mock server
			server := &http.Server{
				Handler: h2cHandler,
			}
			if err := server.Serve(ln); err != nil {
				log.Fatalf("Server on %s failed: %v", port, err)
			}
		})
	}

	wg.Wait()
}

func parseConfig() ([]rule, string) {
	rulesEnv := os.Getenv("FAULT_RULES")

	defaultBehavior := os.Getenv("FAULT_DEFAULT")
	if defaultBehavior == "" {
		defaultBehavior = "200"
	}

	if defaultBehavior != "close" {
		if _, err := strconv.Atoi(defaultBehavior); err != nil {
			log.Fatalf("FAULT_DEFAULT must be a status code or 'close', got: %s", defaultBehavior) //nolint:gosec // env var value is safe to log
		}
	}

	var rules []rule

	var totalPercentage float64

	if rulesEnv != "" {
		for entry := range strings.SplitSeq(rulesEnv, ",") {
			parts := strings.SplitN(entry, ":", ruleParts)
			if len(parts) != ruleParts {
				log.Fatalf("Invalid rule format: %s, expected statusCode:percentage", entry) //nolint:gosec // env var value is safe to log
			}

			statusCode, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				log.Fatalf("Invalid status code in rule: %s: %v", entry, err) //nolint:gosec // env var value is safe to log
			}

			percentage, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err != nil {
				log.Fatalf("Invalid percentage in rule %s: %v", entry, err) //nolint:gosec // env var value is safe to log
			}

			rules = append(rules, rule{statusCode: statusCode, percentage: percentage})
			totalPercentage += percentage
		}

		if totalPercentage > maxPercentage {
			log.Fatalf("Total percentage across rules is %.2f%%, exceeds 100%%", totalPercentage)
		}
	}

	if rulesEnv != "" {
		log.Printf("Fault rules: %s (default: %s, remainder: %.2f%%)", rulesEnv, defaultBehavior, maxPercentage-totalPercentage) //nolint:gosec // env var values are safe to log
	} else {
		log.Printf("No fault rules configured, all requests use default: %s", defaultBehavior) //nolint:gosec // env var value is safe to log
	}

	return rules, defaultBehavior
}

func isGRPC(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc")
}

// writeGRPCResponse writes a proper gRPC response over HTTP/2.
// For success (httpCode 200) it sends an empty OTLP export response body.
// For errors it sends only the grpc-status trailer with no body.
func writeGRPCResponse(w http.ResponseWriter, httpCode int) {
	h := w.Header()
	h.Set("Content-Type", "application/grpc")

	grpcStatus := "0"
	if httpCode != http.StatusOK {
		grpcStatus = strconv.Itoa(httpToGRPCStatus(httpCode))
	}

	// Declare the trailer before WriteHeader so HTTP/2 sends it as a proper trailer frame.
	h.Set("Trailer", "Grpc-Status")

	w.WriteHeader(http.StatusOK)

	if httpCode == http.StatusOK {
		// Write empty gRPC data frame (5-byte header: no compression, 0-byte body).
		frame := make([]byte, grpcFrameHdrSize)
		frame[0] = 0 // no compression
		binary.BigEndian.PutUint32(frame[1:], 0)
		_, _ = w.Write(frame) //nolint:errcheck // best-effort write
	}

	// Set the trailer. Using http.TrailerPrefix works for both HTTP/1.1 and HTTP/2.
	h.Set(http.TrailerPrefix+"Grpc-Status", grpcStatus)
}

func buildHandler(rules []rule, defaultBehavior string, reqStats *stats) http.Handler {
	type threshold struct {
		cumulative float64
		statusCode int
	}

	thresholds := make([]threshold, 0, len(rules))

	var cumulative float64
	for _, r := range rules {
		cumulative += r.percentage
		thresholds = append(thresholds, threshold{cumulative: cumulative, statusCode: r.statusCode})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain the request body to avoid connection resets on keep-alive connections.
		// Without this, responding before the client finishes sending can cause a TCP RST,
		// which the OTLP exporter treats as a retryable connection error instead of a clean HTTP 400.
		_, _ = io.Copy(io.Discard, r.Body) //nolint:errcheck // best-effort drain; body may already be closed

		roll := rand.Float64() * maxPercentage //nolint:gosec // deterministic randomness is fine for fault injection

		for _, t := range thresholds {
			if roll < t.cumulative {
				respond(w, r, t.statusCode)
				reqStats.record(t.statusCode)

				return
			}
		}

		if defaultBehavior == "close" {
			reqStats.record(0)

			hj, ok := w.(http.Hijacker)
			if !ok {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			conn, _, err := hj.Hijack()
			if err != nil {
				return
			}

			conn.Close()

			return
		}

		code, err := strconv.Atoi(defaultBehavior)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		respond(w, r, code)
		reqStats.record(code)
	})
}

// respond writes the appropriate response based on whether the request is gRPC or plain HTTP.
func respond(w http.ResponseWriter, r *http.Request, httpCode int) {
	if isGRPC(r) {
		writeGRPCResponse(w, httpCode)
		return
	}

	w.WriteHeader(httpCode)
}
