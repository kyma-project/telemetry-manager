package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kyma-project/directory-size-exporter/internal/exporter"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultLogFormat                = "json"
	defaultLogLevel                 = "info"
	defaultMetricCollectionInterval = 30 * time.Second
	readHeaderTimeout               = 1 * time.Second
	shutdownTimeout                 = 10 * time.Second
)

var (
	logger = createLogger(defaultLogFormat, defaultLogLevel)

	storagePath string
	metricName  string
	logFormat   string
	logLevel    string
	port        string
	interval    time.Duration
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	flag.StringVar(&logFormat, "log-format", defaultLogFormat, "Log format (json or text)")
	flag.StringVar(&logLevel, "log-level", defaultLogLevel, "Log level (debug, info, warn, error)")

	flag.StringVar(&storagePath, "storage-path", "", "Path to the observed data folder")
	flag.StringVar(&metricName, "metric-name", "", "Metric name used for exporting the folder size")
	flag.StringVar(&port, "port", "2021", "Port for exposing the metrics")
	flag.DurationVar(&interval, "interval", defaultMetricCollectionInterval, "Interval to calculate the metric ")

	flag.Parse()

	if err := validateFlags(); err != nil {
		return fmt.Errorf("invalid flags: %w", err)
	}

	ctx := context.Background()

	logger = createLogger(logFormat, logLevel)
	exp := exporter.NewExporter(storagePath, metricName, logger)

	exp.RecordMetrics(ctx, interval)
	logger.InfoContext(ctx, "Started recording metrics")

	http.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	go func() {
		logger.InfoContext(ctx, "Listening on port '"+port+"'")

		// When Shutdown is called, ListenAndServe will return http.ErrServerClosed, do not log it as an error
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "HTTP server error: %v", slog.Any("err", err))
		}

		logger.InfoContext(ctx, "Stopped serving new connections.")
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	shutdownCtx, shutdownRelease := context.WithTimeout(ctx, shutdownTimeout)
	defer shutdownRelease()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.ErrorContext(shutdownCtx, "HTTP shutdown error: %v", slog.Any("err", err))
		return err
	}

	logger.InfoContext(shutdownCtx, "Graceful shutdown complete.")

	return nil
}

func validateFlags() error {
	if storagePath == "" {
		return errors.New("--storage-path flag is required")
	}

	if metricName == "" {
		return errors.New("--metric-name flag is required")
	}

	if logFormat != "json" && logFormat != "text" {
		return errors.New("--log-format flag should be either 'json' or 'text'")
	}

	if logLevel != "debug" && logLevel != "info" && logLevel != "warn" && logLevel != "error" {
		return errors.New("--log-level flag should be either 'debug', 'info', 'warn' or 'error'")
	}

	return nil
}

func createLogger(logFormat, logLevel string) *slog.Logger {
	level := slog.LevelInfo

	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	var handler slog.Handler

	handlerOpts := slog.HandlerOptions{
		Level: level,
	}
	if logFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &handlerOpts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, &handlerOpts)
	}

	return slog.New(handler)
}
