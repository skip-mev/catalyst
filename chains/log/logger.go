package logging

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type loggerContextKey struct{}

var (
	logFile *os.File
	once    sync.Once
)

func ensureLogDirectory() error {
	const logDir = "/tmp/catalyst"

	//nolint:gosec // G301: valid perm
	if err := os.MkdirAll(logDir, 0o644); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	return nil
}

func getLogFile() (*os.File, error) {
	var err error
	once.Do(func() {
		if err = ensureLogDirectory(); err != nil {
			return
		}

		timestamp := time.Now().Format("2006-01-02-15-04-05")
		logPath := filepath.Join("/tmp/catalyst", fmt.Sprintf("catalyst-%s.log", timestamp))

		//nolint:gosec // G302: valid perm
		logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o655)
		if err != nil {
			err = fmt.Errorf("failed to open log file: %w", err)
			return
		}

		go func() {
			c := make(chan os.Signal, 1)
			<-c
			if logFile != nil {
				_ = logFile.Sync()
				_ = logFile.Close()
			}
			os.Exit(0)
		}()
	})

	return logFile, err
}

func CloseLogFile() {
	if logFile != nil {
		_ = logFile.Sync()
		_ = logFile.Close()
	}
}

func DefaultLogger(devLogging bool, options ...zap.Option) (*zap.Logger, error) {
	var encoder zapcore.Encoder
	var logLevel zapcore.Level

	if devLogging {
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
		logLevel = zap.DebugLevel
	} else {
		encoderConfig := zap.NewProductionEncoderConfig()
		encoder = zapcore.NewJSONEncoder(encoderConfig)
		logLevel = zap.InfoLevel
	}

	logFile, err := getLogFile()
	if err != nil {
		return zap.NewDevelopment(options...)
	}

	stdoutCore := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		logLevel,
	)

	fileCore := zapcore.NewCore(
		encoder,
		zapcore.AddSync(logFile),
		logLevel,
	)

	core := zapcore.NewTee(stdoutCore, fileCore)

	logger := zap.New(core, options...)

	return logger, nil
}

func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

func FromContext(ctx context.Context) *zap.Logger {
	logger, ok := ctx.Value(loggerContextKey{}).(*zap.Logger)
	if ok {
		return logger
	}

	logger, err := DefaultLogger(false)
	if err != nil {
		return zap.NewNop()
	}

	return logger
}

// FieldOnLevel only returns a field if the logger level is at least `level`.
// The function depends on the assumption that the logger level is not going to change in runtime (only set on startup).
func FieldOnLevel(ctx context.Context, level zapcore.Level, field zap.Field) zap.Field {
	if !FromContext(ctx).Core().Enabled(level) {
		return zap.Skip()
	}

	return field
}
