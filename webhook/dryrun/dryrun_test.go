package dryrun

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/webhook/dryrun/mocks"
)

func TestDryRunner_RunPipeline(t *testing.T) {
	testCases := []struct {
		name         string
		prepareErr   error
		runCmdErr    error
		runCmdOutput []byte
		expectedErr  error
	}{
		{
			name:         "success",
			runCmdOutput: []byte("configuration test is successful"),
		},
		{
			name:        "dryrun prepare error",
			prepareErr:  errors.New("prepare error"),
			expectedErr: errors.New("prepare error"),
		},
		{
			name:        "unknown cmd error",
			runCmdErr:   errors.New("unknown error"),
			expectedErr: errors.New("error validating the supplied configuration"),
		},
		{
			name:         "cmd error: invalid flush value",
			runCmdErr:    errors.New("cmd error"),
			runCmdOutput: []byte("[2022/05/24 16:20:55] [\u001b[1m\u001B[91merror\u001B[0m] invalid flush value, aborting."),
			expectedErr:  errors.New("error validating the supplied configuration: invalid flush value, aborting."),
		},
		{
			name:         "cmd error: plugin does not exist",
			runCmdErr:    errors.New("cmd error"),
			runCmdOutput: []byte("[2022/05/24 16:54:56] [error] [config] section 'abc' tried to instance a plugin name that don't exists\n[2022/05/24 16:54:56] [error] configuration file contains errors, aborting."),
			expectedErr:  errors.New("error validating the supplied configuration: section 'abc' tried to instance a plugin name that don't exists"),
		},
		{
			name:         "cmd error: invalid memory buffer limit",
			runCmdErr:    errors.New("cmd error"),
			runCmdOutput: []byte("[2022/05/24 15:56:05] [error] [config] could not configure property 'Mem_Buf_Limit' on input plugin with section name 'tail'\nconfiguration test is successful"),
			expectedErr:  errors.New("error validating the supplied configuration: could not configure property 'Mem_Buf_Limit' on input plugin with section name 'tail'"),
		},
		{
			name:         "cmd error: no parser name",
			runCmdErr:    errors.New("cmd error"),
			runCmdOutput: []byte("[2022/05/24 15:56:05] [error] [parser] no parser 'name' found in file 'custom_parsers.conf'"),
			expectedErr:  errors.New("error validating the supplied configuration: no parser 'name' found"),
		},
		{
			name:         "cmd error: invalid indentation level",
			runCmdErr:    errors.New("cmd error"),
			runCmdOutput: []byte("[2022/05/24 15:59:59] [error] [config] error in dynamic-parsers/parsers.conf:3: invalid indentation level\n"),
			expectedErr:  errors.New("error validating the supplied configuration: invalid indentation level"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockFileWriter := &mocks.FileWriter{}       // Replace with your actual mock implementation
			mockCommandRunner := &mocks.CommandRunner{} // Replace with your actual mock implementation
			config := Config{}                          // Initialize as needed

			dryRunner := &DryRunner{
				fileWriter:    mockFileWriter,
				commandRunner: mockCommandRunner,
				config:        config,
			}

			ctx := context.Background()
			pipeline := &telemetryv1alpha1.LogPipeline{}
			mockFileWriter.On("PreparePipelineDryRun", ctx, mock.Anything, pipeline).Return(func() {}, tc.prepareErr)
			mockCommandRunner.On("Run", ctx, "fluent-bit/bin/fluent-bit",
				"--dry-run",
				"--quiet",
				"--config",
				mock.MatchedBy(func(s string) bool {
					return strings.HasPrefix(s, "/tmp/dry-run-") && strings.HasSuffix(s, "/fluent-bit.conf")
				})).Return(tc.runCmdOutput, tc.runCmdErr)

			err := dryRunner.RunPipeline(ctx, pipeline)
			if tc.expectedErr != nil {
				require.Error(t, err)
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDryRunner_RunParser(t *testing.T) {
	testCases := []struct {
		name         string
		prepareErr   error
		runCmdErr    error
		runCmdOutput []byte
		expectedErr  error
	}{
		{
			name:         "success",
			runCmdOutput: []byte("configuration test is successful"),
		},
		{
			name:        "dryrun prepare error",
			prepareErr:  errors.New("prepare error"),
			expectedErr: errors.New("prepare error"),
		},
		{
			name:        "unknown cmd error",
			runCmdErr:   errors.New("unknown error"),
			expectedErr: errors.New("error validating the supplied configuration"),
		},
		{
			name:         "cmd error: invalid flush value",
			runCmdErr:    errors.New("cmd error"),
			runCmdOutput: []byte("[2022/05/24 16:20:55] [\u001b[1m\u001B[91merror\u001B[0m] invalid flush value, aborting."),
			expectedErr:  errors.New("error validating the supplied configuration: invalid flush value, aborting."),
		},
		{
			name:         "cmd error: plugin does not exist",
			runCmdErr:    errors.New("cmd error"),
			runCmdOutput: []byte("[2022/05/24 16:54:56] [error] [config] section 'abc' tried to instance a plugin name that don't exists\n[2022/05/24 16:54:56] [error] configuration file contains errors, aborting."),
			expectedErr:  errors.New("error validating the supplied configuration: section 'abc' tried to instance a plugin name that don't exists"),
		},
		{
			name:         "cmd error: invalid memory buffer limit",
			runCmdErr:    errors.New("cmd error"),
			runCmdOutput: []byte("[2022/05/24 15:56:05] [error] [config] could not configure property 'Mem_Buf_Limit' on input plugin with section name 'tail'\nconfiguration test is successful"),
			expectedErr:  errors.New("error validating the supplied configuration: could not configure property 'Mem_Buf_Limit' on input plugin with section name 'tail'"),
		},
		{
			name:         "cmd error: no parser name",
			runCmdErr:    errors.New("cmd error"),
			runCmdOutput: []byte("[2022/05/24 15:56:05] [error] [parser] no parser 'name' found in file 'custom_parsers.conf'"),
			expectedErr:  errors.New("error validating the supplied configuration: no parser 'name' found"),
		},
		{
			name:         "cmd error: invalid indentation level",
			runCmdErr:    errors.New("cmd error"),
			runCmdOutput: []byte("[2022/05/24 15:59:59] [error] [config] error in dynamic-parsers/parsers.conf:3: invalid indentation level\n"),
			expectedErr:  errors.New("error validating the supplied configuration: invalid indentation level"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockFileWriter := &mocks.FileWriter{}       // Replace with your actual mock implementation
			mockCommandRunner := &mocks.CommandRunner{} // Replace with your actual mock implementation
			config := Config{}                          // Initialize as needed

			dryRunner := &DryRunner{
				fileWriter:    mockFileWriter,
				commandRunner: mockCommandRunner,
				config:        config,
			}

			ctx := context.Background()
			parser := &telemetryv1alpha1.LogParser{}
			mockFileWriter.On("PrepareParserDryRun", ctx, mock.Anything, parser).Return(func() {}, tc.prepareErr)
			mockCommandRunner.On("Run", ctx, "fluent-bit/bin/fluent-bit",
				"--dry-run",
				"--quiet",
				"--parser",
				mock.MatchedBy(func(s string) bool {
					return strings.HasPrefix(s, "/tmp/dry-run-") && strings.HasSuffix(s, "/dynamic-parsers/parsers.conf")
				})).Return(tc.runCmdOutput, tc.runCmdErr)

			err := dryRunner.RunParser(ctx, parser)
			if tc.expectedErr != nil {
				require.Error(t, err)
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
