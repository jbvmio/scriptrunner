package scriptrunner

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ConfigureLevel returns the appropriate AtomicLevel that can be used for a logger.
func ConfigureLevel(logLevel string) zap.AtomicLevel {
	var level zap.AtomicLevel
	switch strings.ToLower(logLevel) {
	case "", "info":
		level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "debug":
		level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "warn":
		level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "panic":
		level = zap.NewAtomicLevelAt(zap.PanicLevel)
	case "fatal":
		level = zap.NewAtomicLevelAt(zap.FatalLevel)
	default:
		fmt.Printf("Invalid log level supplied. Defaulting to info: %s", logLevel)
		level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	return level
}

// ConfigureLogger return a Logger with the specified loglevel and output.
func ConfigureLogger(level zap.AtomicLevel, ws zapcore.WriteSyncer) *zap.Logger {
	var syncOutput zapcore.WriteSyncer
	// ws := os.Open(`./file`)
	// ws := os.Stdout
	syncOutput = zapcore.Lock(ws)
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		syncOutput,
		level,
	)
	logger := zap.New(core)
	//zap.ReplaceGlobals(logger)
	return logger
}
