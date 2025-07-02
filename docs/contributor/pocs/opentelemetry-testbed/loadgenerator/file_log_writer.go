package main

import (
	"context"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type FileLogWriter struct{}

// Ensure FileLogWriter implements LogDataSender.
var _ testbed.LogDataSender = (*FileLogWriter)(nil)

// NewFileLogWriter creates a new data sender that will write log entries to stdout
func NewFileLogWriter() *FileLogWriter {
	return &FileLogWriter{}
}

func (f *FileLogWriter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (f *FileLogWriter) Start() error {
	return nil
}

func (f *FileLogWriter) ConsumeLogs(_ context.Context, logs plog.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		for j := 0; j < logs.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			ills := logs.ResourceLogs().At(i).ScopeLogs().At(j)
			for k := 0; k < ills.LogRecords().Len(); k++ {
				log.Print(f.convertLogToTextLine(ills.LogRecords().At(k)))
			}
		}
	}
	return nil
}

func (f *FileLogWriter) convertLogToTextLine(lr plog.LogRecord) string {
	sb := strings.Builder{}

	// Timestamp
	sb.WriteString(time.Unix(0, int64(lr.Timestamp())).Format("2006-01-02"))

	// Severity
	sb.WriteString(" ")
	sb.WriteString(lr.SeverityText())
	sb.WriteString(" ")

	if lr.Body().Type() == pcommon.ValueTypeStr {
		sb.WriteString(lr.Body().Str())
	}

	for k, v := range lr.Attributes().All() {
		sb.WriteString(" ")
		sb.WriteString(k)
		sb.WriteString("=")
		switch v.Type() {
		case pcommon.ValueTypeStr:
			sb.WriteString(v.Str())
		case pcommon.ValueTypeInt:
			sb.WriteString(strconv.FormatInt(v.Int(), 10))
		case pcommon.ValueTypeDouble:
			sb.WriteString(strconv.FormatFloat(v.Double(), 'f', -1, 64))
		case pcommon.ValueTypeBool:
			sb.WriteString(strconv.FormatBool(v.Bool()))
		default:
			panic("missing case")
		}
	}

	return sb.String()
}

func (f *FileLogWriter) Flush() {
}

func (f *FileLogWriter) GenConfigYAMLStr() string {
	return ""
}

func (f *FileLogWriter) ProtocolName() string {
	return "filelog"
}

func (f *FileLogWriter) GetEndpoint() net.Addr {
	return nil
}
