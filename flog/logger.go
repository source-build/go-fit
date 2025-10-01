package flog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"time"
)

type Level = int8

// Consistent with zap
const (
	DebugLevel = Level(zapcore.DebugLevel)
	InfoLevel  = Level(zapcore.InfoLevel)
	WarnLevel  = Level(zapcore.WarnLevel)
	ErrorLevel = Level(zapcore.ErrorLevel)
	PanicLevel = Level(zapcore.PanicLevel)
	FatalLevel = Level(zapcore.FatalLevel)
)

type EncoderConfigType string

const (
	// Nil user defined configuration encoding machine
	Nil EncoderConfigType = ""

	DevelopmentEncoderConfig EncoderConfigType = "development"

	ProductionEncoderConfig EncoderConfigType = "production"
)

type LevelEnablerFunc func(Level) bool

type Options struct {
	// A Level is a logging priority. Higher levels are more important.
	LogLevel Level

	EncoderConfigType EncoderConfigType

	// Allows users to configure the concrete encoders supplied by
	EncoderConfig zapcore.EncoderConfig

	CallerSkip int

	// Whether to output to the console
	Console bool

	// Filename is the file to write logs to.  Backup log files will be retained
	// in the same directory.  It uses <processname>-lumberjack.log in
	// os.TempDir() if empty.
	// if the parameter is empty, it will not be written to the log file.
	Filename string

	// MaxSize is the maximum size in megabytes of the log file before it gets
	// rotated. It defaults to 100 megabytes.
	MaxSize int

	// MaxAge is the maximum number of days to retain old log files based on the
	// timestamp encoded in their filename.  Note that a day is defined as 24
	// hours and may not exactly correspond to calendar days due to daylight
	// savings, leap seconds, etc. The default is not to remove old log files
	// based on age.
	MaxAge int

	// MaxBackups is the maximum number of old log files to retain.  The default
	// is to retain all old log files (though MaxAge may still cause them to get
	// deleted.)
	MaxBackups int

	// LocalTime determines if the time used for formatting the timestamps in
	// backup files is the computer's local time.  The default is to use UTC
	// time.
	LocalTime bool

	// Compress determines if the rotated log files should be compressed
	// using gzip. The default is not to perform compression.
	Compress bool

	// Output different logs to different locations
	Tees []TeeOption

	ZapOptions []zap.Option
}

var std *Logger

type Logger struct {
	l  *zap.Logger
	al *zap.AtomicLevel
}

func New(opt Options) *Logger {
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
	opts = append(opts, zap.AddCaller())

	if opt.CallerSkip > 0 {
		opts = append(opts, zap.AddCallerSkip(opt.CallerSkip))
	} else {
		opts = append(opts, zap.AddCallerSkip(2))
	}

	return &Logger{
		l:  zap.New(zapcore.NewTee(cores...), opts...),
		al: &al,
	}
}

func Init(opt Options) {
	std = New(opt)
}

type Field = zap.Field

func (l *Logger) SetLevel(level Level) {
	if l.al != nil {
		l.al.SetLevel(zapcore.Level(level))
	}
}

func (l *Logger) Sugar() *zap.SugaredLogger {
	return l.l.Sugar()
}

func (l *Logger) Debug(msg string, fields ...Field) {
	l.l.Debug(msg, fields...)
}

func (l *Logger) Info(msg string, fields ...Field) {
	l.l.Info(msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...Field) {
	l.l.Warn(msg, fields...)
}

func (l *Logger) Error(msg string, fields ...Field) {
	l.l.Error(msg, fields...)
}

func (l *Logger) Panic(msg string, fields ...Field) {
	l.l.Panic(msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...Field) {
	l.l.Fatal(msg, fields...)
}

func (l *Logger) Sync() error {
	return l.l.Sync()
}

func (l *Logger) Logger() *zap.Logger {
	return l.l
}

func Sugar() *zap.SugaredLogger { return std.Sugar() }

func ZapLogger() *zap.Logger { return std.l }

func Default() *Logger { return std }

func ReplaceDefault(l *Logger) { std = l }

func SetLevel(level Level) { std.SetLevel(level) }

func Debug(msg string, fields ...Field) { std.Debug(msg, fields...) }
func Info(msg string, fields ...Field)  { std.Info(msg, fields...) }
func Warn(msg string, fields ...Field)  { std.Warn(msg, fields...) }
func Error(msg string, fields ...Field) { std.Error(msg, fields...) }
func Panic(msg string, fields ...Field) { std.Panic(msg, fields...) }
func Fatal(msg string, fields ...Field) { std.Fatal(msg, fields...) }

func Sync() error { return std.Sync() }
