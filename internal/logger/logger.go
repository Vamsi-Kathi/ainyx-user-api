package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New constructs a configured Uber Zap logger.
//
// In production it emits JSON logs at the configured level. In development it
// emits human-friendly console logs with colored levels. Callers should defer
// logger.Sync() to flush buffered entries on shutdown.
func New(level string, production bool) (*zap.Logger, error) {
	lvl := parseLevel(level)

	var cfg zap.Config
	if production {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "ts"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	cfg.Level = zap.NewAtomicLevelAt(lvl)

	// AddStacktrace ensures errors carry a stack trace, as required.
	return cfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
}

func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
