package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
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

	portOTLPGRPC  = ":4317"
	portOTLPHTTP  = ":4318"
	portFluentD   = ":9880"
	portConfigAPI = ":9090"
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
	delayMS    int // milliseconds to sleep before responding; 0 means no delay
}

type threshold struct {
	cumulative float64
	statusCode int
	delayMS    int
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

// faultConfig holds the mutable fault injection configuration.
// The data-path handler reads it under RLock; the /config endpoint writes under Lock.
type faultConfig struct {
	mu              sync.RWMutex
	thresholds      []threshold
	defaultBehavior string
	defaultDelayMS  int
}

func newFaultConfig(rules []rule, defaultBehavior string, defaultDelayMS int) *faultConfig {
	return &faultConfig{
		thresholds:      buildThresholds(rules),
		defaultBehavior: defaultBehavior,
		defaultDelayMS:  defaultDelayMS,
	}
}

func buildThresholds(rules []rule) []threshold {
	// Build cumulative thresholds for weighted random selection.
	// E.g. rules 500:30, 429:20 produce thresholds [30, 50]; a roll in [0,30)
	// returns 500, [30,50) returns 429, and >=50 falls through to the default.
	thresholds := make([]threshold, 0, len(rules))

	var cumulative float64
	for _, r := range rules {
		cumulative += r.percentage
		thresholds = append(thresholds, threshold{cumulative: cumulative, statusCode: r.statusCode, delayMS: r.delayMS})
	}

	return thresholds
}

func main() {
	log.SetPrefix(fmt.Sprintf("[%s] ", "mock-backend"))

	rules, defaultBehavior, defaultDelayMS := parseConfig()
	reqStats := newStats(rules, defaultBehavior)
	cfg := newFaultConfig(rules, defaultBehavior, defaultDelayMS)

	go func() {
		ticker := time.NewTicker(statsInterval)
		defer ticker.Stop()

		for range ticker.C {
			reqStats.logAndReset()
		}
	}()

	handler := buildHandler(cfg, reqStats)

	ports := []string{portOTLPGRPC, portOTLPHTTP, portFluentD}

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

	// Start the config endpoint on a separate port so it doesn't interfere with data paths.
	wg.Go(func() {
		mux := http.NewServeMux()
		mux.HandleFunc("POST /config", configHandler(cfg, reqStats))

		var lc net.ListenConfig

		ln, err := lc.Listen(context.Background(), "tcp", portConfigAPI)
		if err != nil {
			log.Fatalf("Failed to listen on %s: %v", portConfigAPI, err)
		}

		log.Printf("Config endpoint listening on %s", portConfigAPI)

		//nolint:gosec // no timeouts needed for test-only mock server
		server := &http.Server{Handler: mux}
		if err := server.Serve(ln); err != nil {
			log.Fatalf("Config server on %s failed: %v", portConfigAPI, err)
		}
	})

	wg.Wait()
}

// configRequest is the JSON body for POST /config.
// All fields are optional; omitted fields keep their current value.
type configRequest struct {
	Rules   *string `json:"rules"`
	Default *string `json:"default"`
	Delays  *string `json:"delays"`
}

// configHandler returns an http.HandlerFunc that reconfigures fault rules at runtime.
func configHandler(cfg *faultConfig, reqStats *stats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req configRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
			return
		}

		cfg.mu.Lock()
		defer cfg.mu.Unlock()

		if req.Default != nil {
			def := *req.Default
			if def != "close" {
				if _, err := strconv.Atoi(def); err != nil {
					http.Error(w, fmt.Sprintf("invalid default: must be a status code or 'close', got: %s", def), http.StatusBadRequest)
					return
				}
			}

			cfg.defaultBehavior = def
		}

		delays := make(map[int]int)

		if req.Delays != nil {
			var err error

			delays, err = parseDelayString(*req.Delays)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid delays: %v", err), http.StatusBadRequest)
				return
			}
		}

		if req.Rules != nil {
			rules, err := parseRulesString(*req.Rules, delays)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid rules: %v", err), http.StatusBadRequest)
				return
			}

			cfg.thresholds = buildThresholds(rules)

			// Rebuild stats to track new status codes.
			*reqStats = *newStats(rules, cfg.defaultBehavior)
		}

		// Update default delay.
		if cfg.defaultBehavior != "close" {
			if code, err := strconv.Atoi(cfg.defaultBehavior); err == nil {
				cfg.defaultDelayMS = delays[code]
			}
		}

		log.Printf("Config updated: default=%s, thresholds=%d", cfg.defaultBehavior, len(cfg.thresholds)) //nolint:gosec // safe to log

		w.WriteHeader(http.StatusOK)
	}
}

// parseRulesString parses the rules format "statusCode:percentage,..." into a []rule.
func parseRulesString(rulesStr string, delays map[int]int) ([]rule, error) {
	if rulesStr == "" {
		return nil, nil
	}

	var (
		rules           []rule
		totalPercentage float64
	)

	for entry := range strings.SplitSeq(rulesStr, ",") {
		parts := strings.SplitN(entry, ":", ruleParts)
		if len(parts) != ruleParts {
			return nil, fmt.Errorf("invalid rule format: %s, expected statusCode:percentage", entry)
		}

		statusCode, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid status code in rule: %s: %w", entry, err)
		}

		percentage, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid percentage in rule %s: %w", entry, err)
		}

		rules = append(rules, rule{statusCode: statusCode, percentage: percentage, delayMS: delays[statusCode]})
		totalPercentage += percentage
	}

	if totalPercentage > maxPercentage {
		return nil, fmt.Errorf("total percentage across rules is %.2f%%, exceeds 100%%", totalPercentage)
	}

	return rules, nil
}

// parseDelayString parses "statusCode:delayMs,..." into map[statusCode]delayMs.
func parseDelayString(env string) (map[int]int, error) {
	delays := make(map[int]int)

	if env == "" {
		return delays, nil
	}

	for entry := range strings.SplitSeq(env, ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", ruleParts)
		if len(parts) != ruleParts {
			return nil, fmt.Errorf("invalid delay format: %s, expected statusCode:delayMs", entry)
		}

		statusCode, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid status code in delay: %s: %w", entry, err)
		}

		delayMS, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid delay ms in delay: %s: %w", entry, err)
		}

		delays[statusCode] = delayMS
	}

	return delays, nil
}

func parseConfig() ([]rule, string, int) {
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

	// Parse per-status-code delays: FAULT_DELAYS=400:500,200:1000 (statusCode:delayMs)
	delays := parseDelays(os.Getenv("FAULT_DELAYS"))

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

			rules = append(rules, rule{statusCode: statusCode, percentage: percentage, delayMS: delays[statusCode]})
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

	if len(delays) > 0 {
		log.Printf("Delays: %s", os.Getenv("FAULT_DELAYS")) //nolint:gosec // env var value is safe to log
	}

	// Look up the delay for the default behavior status code (if any).
	var defaultDelayMS int

	if defaultBehavior != "close" {
		if code, err := strconv.Atoi(defaultBehavior); err == nil {
			defaultDelayMS = delays[code]
		}
	}

	return rules, defaultBehavior, defaultDelayMS
}

// parseDelays parses FAULT_DELAYS env var: "400:500,200:1000" → map[statusCode]delayMs.
func parseDelays(env string) map[int]int {
	delays := make(map[int]int)

	if env == "" {
		return delays
	}

	for entry := range strings.SplitSeq(env, ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", ruleParts)
		if len(parts) != ruleParts {
			log.Fatalf("Invalid delay format: %s, expected statusCode:delayMs", entry) //nolint:gosec // env var value is safe to log
		}

		statusCode, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			log.Fatalf("Invalid status code in delay: %s: %v", entry, err) //nolint:gosec // env var value is safe to log
		}

		delayMS, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			log.Fatalf("Invalid delay ms in delay: %s: %v", entry, err) //nolint:gosec // env var value is safe to log
		}

		delays[statusCode] = delayMS
	}

	return delays
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

func buildHandler(cfg *faultConfig, reqStats *stats) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain the request body to avoid connection resets on keep-alive connections.
		// Without this, responding before the client finishes sending can cause a TCP RST,
		// which the OTLP exporter treats as a retryable connection error instead of a clean HTTP 400.
		_, _ = io.Copy(io.Discard, r.Body) //nolint:errcheck // best-effort drain; body may already be closed

		cfg.mu.RLock()
		thresholds := cfg.thresholds
		defaultBehavior := cfg.defaultBehavior
		defaultDelayMS := cfg.defaultDelayMS
		cfg.mu.RUnlock()

		roll := rand.Float64() * maxPercentage //nolint:gosec // deterministic randomness is fine for fault injection

		for _, t := range thresholds {
			if roll < t.cumulative {
				if t.delayMS > 0 {
					time.Sleep(time.Duration(t.delayMS) * time.Millisecond)
				}

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

		if defaultDelayMS > 0 {
			time.Sleep(time.Duration(defaultDelayMS) * time.Millisecond)
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
