package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ad-engine/internal/config"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

var log core.Logger

func Init(cfg config.LoggerConfig) error {
	formatter := NewCLEFFormatter()

	// options для mtlog.New(...)
	var opts []mtlog.Option

	// ---------------- FILE SINK ----------------
	if cfg.EnableFile && cfg.FilePath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.FilePath), 0o755); err != nil {
			return fmt.Errorf("create logs dir: %w", err)
		}

		bufferSize := cfg.BufferSize
		if cfg.SyncMode {
			bufferSize = 0
		}

		fileSink, err := newFileSink(formatter, cfg, bufferSize)
		if err != nil {
			return err
		}

		var sink core.LogEventSink = fileSink

		if !cfg.SyncMode {
			asyncOpts := sinks.AsyncOptions{
				BufferSize:       cfg.BufferSize,
				OverflowStrategy: sinks.OverflowDropOldest,
				FlushInterval:    time.Duration(cfg.FlushIntervalSec) * time.Second,
				BatchSize:        cfg.BatchSize,
				OnError: func(err error) {
					fmt.Fprintf(os.Stderr, "async file sink error: %v\n", err)
				},
				ShutdownTimeout: 5 * time.Second,
			}
			sink = sinks.NewAsyncSink(fileSink, asyncOpts)
		}

		opts = append(opts, mtlog.WithSink(sink))
	}

	// ---------------- CONSOLE SINK ----------------
	if cfg.EnableConsole {
		consoleSink := sinks.NewConsoleSink()
		var sink core.LogEventSink = consoleSink

		if !cfg.SyncMode {
			asyncOpts := sinks.AsyncOptions{
				BufferSize:       cfg.BufferSize,
				OverflowStrategy: sinks.OverflowDropOldest,
				FlushInterval:    time.Duration(cfg.FlushIntervalSec) * time.Second,
				BatchSize:        cfg.BatchSize,
				OnError: func(err error) {
					fmt.Fprintf(os.Stderr, "async console sink error: %v\n", err)
				},
				ShutdownTimeout: 5 * time.Second,
			}
			sink = sinks.NewAsyncSink(consoleSink, asyncOpts)
		}

		opts = append(opts, mtlog.WithSink(sink))
	}

	// Если ничего не включено
	if len(opts) == 0 {
		opts = append(opts, mtlog.WithConsole())
	}

	// ---------------- ENRICHERS / LEVELS ----------------
	opts = append(opts,
		mtlog.WithTimestamp(),
		mtlog.WithMachineName(),
		mtlog.WithProcessInfo(),
		mtlog.WithCallersInfo(),
		mtlog.WithThreadId(),
		mtlog.WithAutoSourceContext(),
		mtlog.WithCorrelationId("RequestId"),
		mtlog.WithMinimumLevel(levelFromString(cfg.Level)),
		mtlog.WithDynamicLevel(&mtlog.LoggingLevelSwitch{}),
		mtlog.WithCapturing(),
		mtlog.WithCapturingDepth(5),
	)

	log = mtlog.New(opts...)

	return nil
}

func newFileSink(formatter *CLEFFormatter, cfg config.LoggerConfig, bufferSize int) (*sinks.RollingFileSink, error) {
	rollingOpts := sinks.RollingFileOptions{
		FilePath:            cfg.FilePath,
		MaxFileSize:         cfg.MaxFileSize,
		RollingInterval:     sinks.RollingIntervalDaily,
		RetainFileCount:     cfg.MaxBackups,
		CompressRolledFiles: cfg.Compress,
		Formatter:           formatter, // CLEFFormatter implements Format(*LogEvent)
		BufferSize:          bufferSize,
	}
	return sinks.NewRollingFileSink(rollingOpts)
}

func levelFromString(s string) core.LogEventLevel {
	switch strings.ToLower(s) {
	case "verbose", "trace":
		return core.VerboseLevel
	case "debug":
		return core.DebugLevel
	case "info", "information":
		return core.InformationLevel
	case "warn", "warning":
		return core.WarningLevel
	case "error":
		return core.ErrorLevel
	case "fatal":
		return core.FatalLevel
	default:
		return core.InformationLevel
	}
}

func Shutdown() {
	if log == nil {
		return
	}
	// mtlog.Logger реализует Close()?
	if closer, ok := any(log).(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}

func LogDebug(msg string, fields ...any) {
	if log != nil {
		log.Debug(msg, fields...)
	}
}

func LogInfo(msg string, fields ...any) {
	if log != nil {
		log.Information(msg, fields...)
	}
}

func LogWarning(msg string, fields ...any) {
	if log != nil {
		log.Warning(msg, fields...)
	}
}

func LogError(msg string, fields ...any) {
	if log != nil {
		log.Error(msg, fields...)
	}
}

func LogWith(fields ...any) core.Logger {
	if log != nil {
		return log.With(fields...)
	}
	return log
}

func LogInformation(messageTemplate string, args ...any) {
	if log != nil {
		log.Information(messageTemplate, args...)
	}
}
