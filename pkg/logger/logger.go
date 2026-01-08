package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger to provide application-wide logging
type Logger struct {
	*zap.Logger
}

// New creates a new logger with the specified level and format
func New(level, format string) (*Logger, error) {
	var zapConfig zap.Config

	// Configure based on format
	if format == "json" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Set log level
	zapLevel, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	zapConfig.Level = zap.NewAtomicLevelAt(zapLevel)

	// Configure time encoding
	zapConfig.EncoderConfig.TimeKey = "timestamp"
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Build logger
	zapLogger, err := zapConfig.Build(
		zap.AddCallerSkip(1), // Skip one level to show correct caller
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return &Logger{Logger: zapLogger}, nil
}

// parseLevel converts string level to zapcore.Level
func parseLevel(level string) (zapcore.Level, error) {
	switch level {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("invalid log level: %s", level)
	}
}

// WithFields returns a logger with the specified fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for key, value := range fields {
		zapFields = append(zapFields, zap.Any(key, value))
	}
	return &Logger{Logger: l.With(zapFields...)}
}

// WithError returns a logger with an error field
func (l *Logger) WithError(err error) *Logger {
	return &Logger{Logger: l.With(zap.Error(err))}
}

// WithComponent returns a logger with a component field
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{Logger: l.With(zap.String("component", component))}
}

// Named returns a logger with a name
func (l *Logger) Named(name string) *Logger {
	return &Logger{Logger: l.Logger.Named(name)}
}

// Info logs a message with key-value pairs
func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.Logger.Info(msg, convertToFields(keysAndValues)...)
}

// Debug logs a debug message with key-value pairs
func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.Logger.Debug(msg, convertToFields(keysAndValues)...)
}

// Warn logs a warning message with key-value pairs
func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	l.Logger.Warn(msg, convertToFields(keysAndValues)...)
}

// Error logs an error message with key-value pairs
func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	l.Logger.Error(msg, convertToFields(keysAndValues)...)
}

// Fatal logs a fatal message with key-value pairs and exits
func (l *Logger) Fatal(msg string, keysAndValues ...interface{}) {
	l.Logger.Fatal(msg, convertToFields(keysAndValues)...)
}

// convertToFields converts alternating key-value pairs to zap.Fields
func convertToFields(keysAndValues []interface{}) []zap.Field {
	if len(keysAndValues) == 0 {
		return nil
	}

	fields := make([]zap.Field, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 >= len(keysAndValues) {
			// Odd number of arguments, add the last key with nil value
			fields = append(fields, zap.Any(fmt.Sprint(keysAndValues[i]), nil))
			break
		}

		key := fmt.Sprint(keysAndValues[i])
		value := keysAndValues[i+1]

		// Type switch for common types to use appropriate zap field constructors
		switch v := value.(type) {
		case string:
			fields = append(fields, zap.String(key, v))
		case int:
			fields = append(fields, zap.Int(key, v))
		case int64:
			fields = append(fields, zap.Int64(key, v))
		case uint:
			fields = append(fields, zap.Uint(key, v))
		case uint64:
			fields = append(fields, zap.Uint64(key, v))
		case float64:
			fields = append(fields, zap.Float64(key, v))
		case bool:
			fields = append(fields, zap.Bool(key, v))
		case error:
			fields = append(fields, zap.Error(v))
		default:
			fields = append(fields, zap.Any(key, v))
		}
	}

	return fields
}
