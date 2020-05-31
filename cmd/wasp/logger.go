package main

import (
	"os"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func getLogger(config *viper.Viper) *zap.Logger {
	var logger *zap.Logger
	var err error

	var level zapcore.Level = zap.InfoLevel
	switch config.GetString("log-level") {
	case "debug":
		level = zap.DebugLevel
	case "error":
		level = zap.ErrorLevel
	case "err":
		level = zap.ErrorLevel
	case "warning":
		level = zap.WarnLevel
	case "warn":
		level = zap.WarnLevel
	}
	fields := []zap.Field{
		zap.String("version", BuiltVersion),
		zap.Time("started_at", time.Now()),
	}
	if allocID := os.Getenv("NOMAD_ALLOC_ID"); allocID != "" {
		fields = append(fields,
			zap.String("nomad_alloc_id", os.Getenv("NOMAD_ALLOC_ID")[:8]),
			zap.String("nomad_alloc_name", os.Getenv("NOMAD_ALLOC_NAME")),
			zap.String("nomad_alloc_index", os.Getenv("NOMAD_ALLOC_INDEX")),
		)
	}
	opts := []zap.Option{
		zap.Fields(fields...),
	}
	if config.GetBool("fancy-logs") {
		config := zap.NewDevelopmentEncoderConfig()
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
		logger = zap.New(zapcore.NewCore(
			zapcore.NewConsoleEncoder(config),
			zapcore.AddSync(colorable.NewColorableStdout()),
			level,
		))
		logger.Debug("started debug logger")
	} else {
		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(level)
		logger, err = config.Build(opts...)
	}
	if err != nil {
		panic(err)
	}
	return logger
}
