package exporter

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initExporterAndRecordMetrics(ctx context.Context, path string) {
	exp := NewExporter(path, "test_metric", slog.Default())

	exp.RecordMetrics(ctx, 5)

	http.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:              ":2021",
		ReadHeaderTimeout: 1 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}

func getMetrics(port int) (map[string]string, error) {
	metrics := map[string]string{}

	res, err := http.Get("http://localhost:" + fmt.Sprint(port) + "/metrics") //nolint:noctx // no need for context here
	if err != nil {
		return metrics, err
	}

	defer res.Body.Close()

	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		lineMetrics := strings.Split(line, " ")
		if len(lineMetrics) < 2 {
			continue
		}

		metrics[lineMetrics[0]] = lineMetrics[1]
	}

	return metrics, err
}

func prepareMockDirectories(testDir string) (string, error) {
	dirPath := testDir + "/test-data"

	err := os.Mkdir(dirPath, 0700)
	if err != nil {
		return "", err
	}

	directories := []string{"dir1", "dir2", "dir3"}
	for i, dirName := range directories {
		err = prepareMockDirectory(dirPath, dirName, int64(i*100))
		if err != nil {
			return "", err
		}
	}

	return dirPath, err
}

func prepareMockDirectory(dirPath string, dirName string, size int64) error {
	const fileName string = "test.txt"

	err := os.Mkdir(dirPath+"/"+dirName, 0700)
	if err != nil {
		return err
	}

	err = writeMockFileToDirectory(dirPath+"/"+dirName, fileName, size)

	return err
}

func writeMockFileToDirectory(dirPath string, filename string, size int64) error {
	newFile, err := os.Create(dirPath + "/" + filename)
	if err != nil {
		return err
	}

	err = os.Truncate(dirPath+"/"+filename, size)
	if err != nil {
		newFile.Close()
		return err
	}

	newFile.Close()

	return err
}

func TestListDir(t *testing.T) {
	dirPath, errDirs := prepareMockDirectories(t.TempDir())
	assert.NoError(t, errDirs)

	expectedDirectories := []directory{
		{name: "dir1", size: int64(0)},
		{name: "dir2", size: int64(100)},
		{name: "dir3", size: int64(200)},
	}

	directories, err := listDirs(dirPath)
	assert.NoError(t, err)

	isTrue := (len(directories) == len(expectedDirectories))

	for i, dir := range directories {
		if dir != expectedDirectories[i] {
			isTrue = false
			break
		}
	}

	require.True(t, isTrue)
}

func TestDirSize(t *testing.T) {
	dirPath, errDirs := prepareMockDirectories(t.TempDir())
	assert.NoError(t, errDirs)

	size, err := dirSize(dirPath)
	assert.NoError(t, err)

	require.Equal(t, int64(300), size)
}

func TestNewExporter(t *testing.T) {
	exporter := NewExporter("data/log", "metric_name", nil)
	require.NotNil(t, exporter)
}

func TestRecordMetric(t *testing.T) {
	dirPath, err := prepareMockDirectories(t.TempDir())
	require.NoError(t, err)

	go initExporterAndRecordMetrics(t.Context(), dirPath)

	time.Sleep(10 * time.Second)

	initialMetrics, err := getMetrics(2021)
	require.NoError(t, err)

	directories, err := os.ReadDir(dirPath)
	require.NoError(t, err)

	emitterMetricInitialValue, prs := initialMetrics["test_metric{directory=\""+directories[0].Name()+"\"}"]
	require.True(t, prs)

	err = writeMockFileToDirectory(dirPath+"/"+directories[0].Name(), "main_test.txt", 500)
	require.NoError(t, err)
	time.Sleep(10 * time.Second)

	metrics, err := getMetrics(2021)
	require.NoError(t, err)

	emitterMetricValue, prs := metrics["test_metric{directory=\""+directories[0].Name()+"\"}"]
	require.True(t, prs)

	require.NotEqual(t, emitterMetricInitialValue, emitterMetricValue)
	require.Equal(t, "500", emitterMetricValue)
}
