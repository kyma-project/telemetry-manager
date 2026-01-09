package exporter

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Exporter interface {
	RecordMetrics(ctx context.Context, interval time.Duration)
}

type exporter struct {
	DirectoriesLabelsVector *prometheus.GaugeVec
	LogPath                 string
	Logger                  *slog.Logger
}

type directory struct {
	name string
	size int64
}

func NewExporter(dataPath string, metricName string, logger *slog.Logger) Exporter {
	metricsGague := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "",
		Subsystem: "",
		Name:      metricName,
		Help:      "Folder sizes of sub-directories",
	}, []string{"directory"})

	return &exporter{
		DirectoriesLabelsVector: metricsGague,
		LogPath:                 dataPath,
		Logger:                  logger,
	}
}

func (v *exporter) RecordMetrics(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	quit := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				v.recordingIteration(ctx, v.LogPath)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func (v *exporter) recordingIteration(ctx context.Context, logPath string) {
	directories, err := listDirs(logPath)
	if err != nil && v.Logger != nil {
		v.Logger.ErrorContext(ctx, "Error listing directories", slog.Any("err", err))
	}

	for _, dir := range directories {
		v.DirectoriesLabelsVector.WithLabelValues(dir.name).Set(float64(dir.size))
	}
}

// dirSize calculates the total size of all regular files in a directory tree.
// It explicitly skips symbolic links to avoid CVE-2024-8244 (TOCTOU vulnerability in filepath.Walk).
func dirSize(path string) (int64, error) {
	var size int64

	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		// Explicitly skip symbolic links to prevent TOCTOU attacks (CVE-2024-8244)
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		fullPath := filepath.Join(path, entry.Name())

		if entry.IsDir() {
			// Recursively calculate subdirectory size
			subSize, err := dirSize(fullPath)
			if err != nil {
				// Log error but continue processing other directories
				continue
			}

			size += subSize
		} else {
			// Get file info for regular files
			info, err := entry.Info()
			if err != nil {
				continue
			}

			if info.Mode().IsRegular() {
				size += info.Size()
			}
		}
	}

	return size, nil
}

func listDirs(path string) ([]directory, error) {
	directories := make([]directory, 0)

	files, err := os.ReadDir(path)
	if err != nil {
		return directories, err
	}

	for _, file := range files {
		// Skip symbolic links to prevent security issues
		if file.Type()&os.ModeSymlink != 0 {
			continue
		}

		if !file.IsDir() {
			continue
		}

		size, err := dirSize(filepath.Join(path, file.Name()))
		if err != nil {
			// Skip directories that cannot be read, but continue processing others
			continue
		}

		directories = append(directories, directory{file.Name(), size})
	}

	return directories, nil
}
