package dryrun

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
)

func dryRunArgs() []string {
	return []string{"--dry-run", "--quiet"}
}

const (
	fluentBitPath = "fluent-bit/bin/fluent-bit"
)

type Config struct {
	FluentBitConfigMapName types.NamespacedName
	PipelineDefaults       builder.PipelineDefaults
}

type DryRunner struct {
	fileWriter    fileWriter
	commandRunner commandRunner
	config        Config
}

func NewDryRunner(c client.Client, config Config) *DryRunner {
	return &DryRunner{
		fileWriter:    &fileWriterImpl{client: c, config: config},
		commandRunner: &commandRunnerImpl{},
		config:        config,
	}
}

func (d *DryRunner) RunParser(ctx context.Context, parser *telemetryv1alpha1.LogParser) error {
	workDir := newWorkDirPath()
	cleanup, err := d.fileWriter.PrepareParserDryRun(ctx, workDir, parser)
	if err != nil {
		return err
	}
	defer cleanup()

	path := filepath.Join(workDir, "dynamic-parsers", "parsers.conf")
	args := append(dryRunArgs(), "--parser", path)
	return d.runCmd(ctx, args)
}

func (d *DryRunner) RunPipeline(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	workDir := newWorkDirPath()
	cleanup, err := d.fileWriter.PreparePipelineDryRun(ctx, workDir, pipeline)
	if err != nil {
		return err
	}
	defer cleanup()

	path := filepath.Join(workDir, "fluent-bit.conf")
	args := dryRunArgs()
	args = append(args, "--config", path)

	return d.runCmd(ctx, args)
}

func (d *DryRunner) runCmd(ctx context.Context, args []string) error {
	outBytes, err := d.commandRunner.Run(ctx, fluentBitPath, args...)
	out := string(outBytes)
	if err != nil {
		if strings.Contains(out, "error") || strings.Contains(out, "Error") {
			return fmt.Errorf("error validating the supplied configuration: %s", extractError(out))
		}
		return errors.New("error validating the supplied configuration")
	}

	return nil
}

func newWorkDirPath() string {
	return "/tmp/dry-run-" + uuid.New().String()
}

func extractError(output string) string {
	// Found in https://github.com/acarl005/stripansi/blob/master/stripansi.go#L7
	rColors := regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\a|(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~])")
	output = rColors.ReplaceAllString(output, "")
	// Error pattern: [<time>] [error] [config] error in path/to/file.conf:3: <msg>
	r1 := regexp.MustCompile(`.*(?P<label>\[error]\s\[config].+:\s)(?P<description>.+)`)
	if r1Matches := r1.FindStringSubmatch(output); r1Matches != nil {
		return r1Matches[2] // 0: complete output, 1: label, 2: description
	}
	// Error pattern: [<time>] [error] [config] <msg>
	r2 := regexp.MustCompile(`.*(?P<label>\[error]\s\[config]\s)(?P<description>.+)`)
	if r2Matches := r2.FindStringSubmatch(output); r2Matches != nil {
		return r2Matches[2] // 0: complete output, 1: label, 2: description
	}
	// Error pattern: [<time>] [error] [parser] <msg> in path/to/file.conf
	r3 := regexp.MustCompile(`.*(?P<label>\[error]\s\[parser]\s)(?P<description>.+)(\sin.+)`)
	if r3Matches := r3.FindStringSubmatch(output); r3Matches != nil {
		return r3Matches[2] // 0: complete output, 1: label, 2: description 3: file name
	}
	// Error pattern: [<time>] [error] <msg>
	r4 := regexp.MustCompile(`.*(?P<label>\[error]\s)(?P<description>.+)`)
	if r4Matches := r4.FindStringSubmatch(output); r4Matches != nil {
		return r4Matches[2] // 0: complete output, 1: label, 2: description
	}
	// Error pattern: error<msg>
	r5 := regexp.MustCompile(`error.+`)
	return r5.FindString(output)
}
