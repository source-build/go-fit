package fit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/source-build/go-fit/flog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

const (
	traceStr     = "%s [%.3fms] [rows:%v] %s"
	traceWarnStr = "%s %s [%.3fms] [rows:%v] %s"
	traceErrStr  = "%s %s [%.3fms] [rows:%v] %s"
)

type GormZapLoggerOption struct {
	// Slow SQL threshold
	SlowThreshold time.Duration

	// Ignore the error of not finding records
	IgnoreRecordNotFoundError bool

	// Disable color printing
	DisableColorful bool

	// CallerPathDepth controls how many trailing path segments are kept in GORM log callers.
	// 0 keeps the original full path; 1 keeps only the file name; 2 keeps the parent directory and file name.
	CallerPathDepth int

	// Structured writes GORM query details as Zap fields instead of formatting them into msg.
	Structured bool

	// ParameterizedQueries omits bind values from the logged SQL statement.
	ParameterizedQueries bool
}

type GormZapLogger struct {
	slowThreshold time.Duration

	Logger *flog.Logger

	logLevel logger.LogLevel

	ignoreRecordNotFoundError bool

	callerPathDepth int

	structured bool

	parameterizedQueries bool

	traceStr string

	traceWarnStr string

	traceErrStr string
}

func NewGormZapLogger(log *flog.Logger, opt ...GormZapLoggerOption) GormZapLogger {
	var logLevel logger.LogLevel
	level := log.Logger().Level()
	switch level {
	case zapcore.InfoLevel:
		logLevel = logger.Info
	case zapcore.DebugLevel:
		logLevel = logger.Info
	case zapcore.WarnLevel:
		logLevel = logger.Warn
	case zapcore.ErrorLevel:
		logLevel = logger.Error
	case zapcore.DPanicLevel:
		logLevel = logger.Error
	case zapcore.PanicLevel:
		logLevel = logger.Error
	case zapcore.FatalLevel:
		logLevel = logger.Error
	case zapcore.InvalidLevel:
		logLevel = logger.Error
	}

	g := GormZapLogger{
		Logger:        log,
		logLevel:      logLevel,
		slowThreshold: 200 * time.Millisecond,
		traceStr:      traceStr,
		traceWarnStr:  traceWarnStr,
		traceErrStr:   traceErrStr,
	}

	if len(opt) > 0 {
		if opt[0].SlowThreshold > 0 {
			g.slowThreshold = opt[0].SlowThreshold
		}

		if opt[0].IgnoreRecordNotFoundError {
			g.ignoreRecordNotFoundError = opt[0].IgnoreRecordNotFoundError
		}

		if opt[0].CallerPathDepth > 0 {
			g.callerPathDepth = opt[0].CallerPathDepth
		}

		g.structured = opt[0].Structured
		g.parameterizedQueries = opt[0].ParameterizedQueries

		if !opt[0].DisableColorful {
			g.traceStr = logger.Green + "%s " + logger.Reset + logger.Yellow + "[%.3fms] " + logger.BlueBold + "[rows:%v]" + logger.Reset + " %s" + "\n"
			g.traceWarnStr = logger.Green + "%s " + logger.Yellow + "%s " + logger.Reset + logger.RedBold + "[%.3fms] " + logger.Yellow + "[rows:%v]" + logger.Magenta + " %s" + logger.Reset + "\n"
			g.traceErrStr = logger.RedBold + "%s " + logger.MagentaBold + "%s " + logger.Reset + logger.Yellow + "[%.3fms] " + logger.BlueBold + "[rows:%v]" + logger.Reset + " %s" + "\n"
		}
	}

	return g
}

func (g GormZapLogger) LogMode(level logger.LogLevel) logger.Interface {
	g.logLevel = level
	switch level {
	case logger.Silent:
	case logger.Error:
		g.Logger.SetLevel(flog.ErrorLevel)
	case logger.Warn:
		g.Logger.SetLevel(flog.WarnLevel)
	case logger.Info:
		g.Logger.SetLevel(flog.InfoLevel)
	}

	return &g
}

func (g GormZapLogger) Info(ctx context.Context, s string, i ...interface{}) {
	g.Logger.Sugar().Infof(s, i)
}

func (g GormZapLogger) Warn(ctx context.Context, s string, i ...interface{}) {
	g.Logger.Sugar().Warnf(s, i)
}

func (g GormZapLogger) Error(ctx context.Context, s string, i ...interface{}) {
	g.Logger.Sugar().Errorf(s, i)
}

func (g GormZapLogger) ParamsFilter(ctx context.Context, sql string, params ...interface{}) (string, []interface{}) {
	if g.parameterizedQueries {
		return sql, nil
	}
	return sql, params
}

func trimGormCallerPath(caller string, depth int) string {
	if depth <= 0 || caller == "" {
		return caller
	}

	parts := strings.Split(strings.Trim(strings.ReplaceAll(caller, "\\", "/"), "/"), "/")
	if len(parts) <= depth {
		return caller
	}
	return strings.Join(parts[len(parts)-depth:], "/")
}

func (g GormZapLogger) traceStructured(caller string, elapsed time.Duration, fc func() (sql string, rowsAffected int64), err error) {
	durationMS := float64(elapsed.Nanoseconds()) / 1e6

	if err != nil && g.logLevel >= logger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !g.ignoreRecordNotFoundError) {
		sql, rows := fc()
		g.Logger.Error("gorm query failed",
			zap.String("source", caller),
			zap.Float64("duration_ms", durationMS),
			zap.Int64("rows", rows),
			zap.String("sql", sql),
			zap.Error(err),
		)
		return
	}

	if elapsed > g.slowThreshold && g.slowThreshold != 0 && g.logLevel >= logger.Warn {
		sql, rows := fc()
		g.Logger.Warn("gorm slow query",
			zap.String("source", caller),
			zap.Float64("duration_ms", durationMS),
			zap.Float64("slow_threshold_ms", float64(g.slowThreshold.Nanoseconds())/1e6),
			zap.Int64("rows", rows),
			zap.String("sql", sql),
		)
		return
	}

	if g.logLevel == logger.Info {
		sql, rows := fc()
		g.Logger.Info("gorm query",
			zap.String("source", caller),
			zap.Float64("duration_ms", durationMS),
			zap.Int64("rows", rows),
			zap.String("sql", sql),
		)
	}
}

func (g GormZapLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if g.logLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	if g.structured {
		g.traceStructured(trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), elapsed, fc, err)
		return
	}

	// An error occurred
	if err != nil && g.logLevel >= logger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !g.ignoreRecordNotFoundError) {
		sql, rows := fc()
		if rows == -1 {
			fmt.Printf(g.traceErrStr, trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			g.Logger.Sugar().Errorf(traceErrStr, trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			fmt.Printf(g.traceErrStr, trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			g.Logger.Sugar().Errorf(traceErrStr, trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}

	// Slow SQL printing, log level needs to be greater than or equal to Warn, and query time needs to be greater than the threshold.
	if elapsed > g.slowThreshold && g.slowThreshold != 0 && g.logLevel >= logger.Warn {
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", g.slowThreshold)
		if rows == -1 {
			fmt.Printf(g.traceWarnStr, trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			g.Logger.Sugar().Warnf(traceWarnStr, trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			fmt.Printf(g.traceWarnStr, trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			g.Logger.Sugar().Warnf(traceWarnStr, trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}

	// When the log level is info, print the complete SQL statement.
	if g.logLevel == logger.Info {
		sql, rows := fc()
		if rows == -1 {
			fmt.Printf(g.traceStr, trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), float64(elapsed.Nanoseconds())/1e6, "-", sql)
			g.Logger.Sugar().Infof(traceStr, trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			fmt.Printf(g.traceStr, trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), float64(elapsed.Nanoseconds())/1e6, rows, sql)
			g.Logger.Sugar().Infof(traceStr, trimGormCallerPath(utils.FileWithLineNum(), g.callerPathDepth), float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}
}
