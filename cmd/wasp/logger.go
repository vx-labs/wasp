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
	if config.GetBool("debug") {
		config := zap.NewDevelopmentEncoderConfig()
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
		logger = zap.New(zapcore.NewCore(
			zapcore.NewConsoleEncoder(config),
			zapcore.AddSync(colorable.NewColorableStdout()),
			zapcore.DebugLevel,
		))
		logger.Debug("started debug logger")
	} else {
		logger, err = zap.NewProduction(opts...)
	}
	if err != nil {
		panic(err)
	}
	return logger
}
