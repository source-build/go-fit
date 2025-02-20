package flog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"time"
)

func NewGormLogger(opt Options) *Logger {
	var cfg zapcore.EncoderConfig
	switch opt.EncoderConfigType {
	case Nil:
		cfg = opt.EncoderConfig
	case ProductionEncoderConfig:
		cfg = zap.NewProductionEncoderConfig()
	case DevelopmentEncoderConfig:
		cfg = zap.NewDevelopmentEncoderConfig()
	default:
		cfg = zap.NewProductionEncoderConfig()
	}

	if opt.EncoderConfigType != Nil {
		cfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format("2006-01-02 15:04:05"))
		}
	}

	al := zap.NewAtomicLevelAt(zapcore.Level(opt.LogLevel))

	cores := NewTee(opt.Tees, cfg)

	if opt.Filename != "" {
		syncer := zapcore.AddSync(&lumberjack.Logger{
			Filename:   opt.Filename,
			MaxSize:    opt.MaxSize,
			MaxBackups: opt.MaxBackups,
			MaxAge:     opt.MaxAge,
			LocalTime:  opt.LocalTime,
			Compress:   opt.Compress,
		})
		cores = append(cores, zapcore.NewCore(zapcore.NewJSONEncoder(cfg), syncer, al))
	}

	if opt.Console {
		syncer := zapcore.AddSync(os.Stdout)
		cores = append(cores, zapcore.NewCore(zapcore.NewJSONEncoder(cfg), syncer, al))
	}

	opts := opt.ZapOptions

	return &Logger{
		l:  zap.New(zapcore.NewTee(cores...), opts...),
		al: &al,
	}
}
