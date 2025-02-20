package fit

import (
	"context"
	"errors"
	"fmt"
	"github.com/source-build/go-fit/flog"
	"go.uber.org/zap/zapcore"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
	"net/url"
	"time"
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
}

type GormZapLogger struct {
	slowThreshold time.Duration

	Logger *flog.Logger

	logLevel logger.LogLevel

	ignoreRecordNotFoundError bool

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

func (g GormZapLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if g.logLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)

	// An error occurred
	if err != nil && g.logLevel >= logger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !g.ignoreRecordNotFoundError) {
		sql, rows := fc()
		if rows == -1 {
			fmt.Printf(g.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			g.Logger.Sugar().Errorf(traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			fmt.Printf(g.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			g.Logger.Sugar().Errorf(traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}

	// Slow SQL printing, log level needs to be greater than or equal to Warn, and query time needs to be greater than the threshold.
	if elapsed > g.slowThreshold && g.slowThreshold != 0 && g.logLevel >= logger.Warn {
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", g.slowThreshold)
		if rows == -1 {
			fmt.Printf(g.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			g.Logger.Sugar().Warnf(traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			fmt.Printf(g.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			g.Logger.Sugar().Warnf(traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}

	// When the log level is info, print the complete SQL statement.
	if g.logLevel == logger.Info {
		sql, rows := fc()
		if rows == -1 {
			fmt.Printf(g.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql)
			g.Logger.Sugar().Infof(traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			fmt.Printf(g.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
			g.Logger.Sugar().Infof(traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}
}

type MySQLClientOption struct {
	Username string

	Password string

	// default tcp
	Protocol string

	Address string

	// Can be empty
	DbName string

	// default：“charset=utf8&parseTime=True&loc=Local”
	Params url.Values

	// Do not use connection pool
	DisableConnPool bool

	// Set the maximum number of idle connections
	MaxIdleConns int

	// Set the maximum number of open connections
	MaxOpenConns int

	// Set the maximum lifetime of the connection
	ConnMaxLifetime time.Duration

	Config *gorm.Config
}

// DB mysql connect
var DB *gorm.DB

// NewMySQLDefaultClient Create a quick MySQL client that only requires commonly used configurations.
// For custom configurations, please use InjectMySQLClient
func NewMySQLDefaultClient(opt MySQLClientOption) error {
	if opt.Username == "" || opt.Password == "" || opt.Address == "" {
		panic("Missing necessary parameters when initializing MySQL")
	}

	if opt.Protocol == "" {
		opt.Protocol = "tcp"
	}

	if opt.Params == nil {
		opt.Params = url.Values{}
		opt.Params.Set("charset", "utf8mb4")
		opt.Params.Set("parseTime", "True")
		opt.Params.Set("loc", "Local")
	}

	config := opt.Config
	if opt.Config == nil {
		config = &gorm.Config{}
	}

	var err error
	DB, err = gorm.Open(
		mysql.Open(fmt.Sprintf("%s:%s@%s(%s)/%s?%s", opt.Username, opt.Password, opt.Protocol, opt.Address, opt.DbName, opt.Params.Encode())),
		config)
	if err != nil {
		return err
	}

	if opt.DisableConnPool {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	if opt.MaxIdleConns == 0 {
		opt.MaxIdleConns = 10
	}

	sqlDB.SetMaxIdleConns(opt.MaxIdleConns)

	if opt.MaxOpenConns == 0 {
		opt.MaxOpenConns = 100
	}

	sqlDB.SetMaxOpenConns(opt.MaxOpenConns)

	if opt.ConnMaxLifetime == 0 {
		opt.ConnMaxLifetime = time.Hour
	}

	sqlDB.SetConnMaxLifetime(opt.ConnMaxLifetime)

	return nil
}

func InjectMySQLClient(db *gorm.DB) {
	DB = db
}

// Model Encapsulated Gorm Model, Mainly added JSON format,
// if there is no such requirement, you can directly use Gorm Model
type Model struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

func HandleGormQueryErrorFromTx(tx *gorm.DB) (*gorm.DB, error) {
	return tx, HandleGormQueryError(tx.Error)
}

func HandleGormQueryError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}

	return err
}
