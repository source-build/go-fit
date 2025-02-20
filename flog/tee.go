package flog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
)

type TeeOption struct {
	Out io.Writer
	LevelEnablerFunc
}

func NewTee(tees []TeeOption, cfg ...zapcore.EncoderConfig) []zapcore.Core {
	var g zapcore.EncoderConfig
	if len(cfg) > 0 {
		g = cfg[0]
	} else {
		g = zap.NewProductionEncoderConfig()
	}

	var cores []zapcore.Core

	for _, tee := range tees {
		var core zapcore.Core
		if tee.LevelEnablerFunc == nil {
			core = zapcore.NewCore(zapcore.NewJSONEncoder(g), zapcore.AddSync(tee.Out), zap.NewAtomicLevelAt(zapcore.Level(InfoLevel)))
		} else {
			core = zapcore.NewCore(zapcore.NewJSONEncoder(g), zapcore.AddSync(tee.Out), zap.LevelEnablerFunc(func(level zapcore.Level) bool {
				return tee.LevelEnablerFunc(Level(level))
			}))
		}

		cores = append(cores, core)
	}

	return cores
}
