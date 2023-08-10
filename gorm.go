package fit

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"time"
)

var mysqlClient *gorm.DB

type DefaultConfigMysql struct {
	User            string
	Pass            string
	IP              string
	Port            string
	DB              string
	FitLogger       *logrus.Logger
	Logger          logger.Interface
	LogMode         logger.LogLevel
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

var sqlDB *sql.DB

func NewMysqlDefConnect(config DefaultConfigMysql, useTrace bool) error {
	if config.MaxIdleConns <= 0 {
		config.MaxIdleConns = 25
	}

	if config.MaxOpenConns <= 0 {
		config.MaxOpenConns = 200
	}

	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = time.Hour
	}

	dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?charset=utf8mb4&parseTime=True&loc=Local", config.User, config.Pass, config.IP, config.Port, config.DB)
	cf := gorm.Config{}
	if config.LogMode == 0 {
		config.LogMode = logger.Error
	}

	if config.FitLogger != nil {
		logLevel := logger.Silent
		switch globalLogLevel {
		case DebugLevel:
			logLevel = logger.Info
		}

		cf.Logger = logger.New(
			config.FitLogger,
			logger.Config{
				LogLevel: logLevel,
			},
		)
	}

	if config.Logger != nil {
		cf.Logger = config.Logger
		cf.Logger.LogMode(config.LogMode)
	}

	client, err := gorm.Open(mysql.Open(dsn), &cf)
	if err != nil {
		return err
	}

	if useTrace {
		if err = client.Use(new(TracePlugin)); err != nil {
			return err
		}
	}

	mysqlClient = client

	// connection pool,use default config
	sqlDB, err = client.DB()
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	if err != nil {
		return err
	}

	return nil
}

func CloseSqlDB() {
	if sqlDB != nil {
		if err := sqlDB.Close(); err != nil {
			Error(err)
		}
	}
}

// NewMysqlConnect  init new mysql_gorm client
// param: addr mysql_gorm address, format: root:123@tcp(127.0.0.1:3369)/foo?charset=utf8mb4&parseTime=True&loc=Local
// param: config mysql_gorm config
func NewMysqlConnect(addr string, config *gorm.Config, useTrace bool) (*sql.DB, error) {
	client, err := gorm.Open(mysql.Open(addr), config)
	if err != nil {
		return nil, err
	}

	if useTrace {
		err = client.Use(new(TracePlugin))
		if err != nil {
			return nil, err
		}
	}

	mysqlClient = client
	sqlDB, err = client.DB()
	if err != nil {
		return nil, err
	}

	return sqlDB, nil
}

func MainMysql() *gorm.DB {
	return mysqlClient
}

func before(db *gorm.DB) {
	db.InstanceSet("startTime", time.Now())
	return
}

type TracePlugin struct{}

func (t TracePlugin) Name() string {
	return "tracePlugin"
}

func (t TracePlugin) Initialize(db *gorm.DB) error {
	// start
	_ = db.Callback().Create().Before("gorm:before_create").Register("before_create", before)
	_ = db.Callback().Query().Before("gorm:query").Register("query", before)
	_ = db.Callback().Delete().Before("gorm:before_delete").Register("before_delete", before)
	_ = db.Callback().Update().Before("gorm:setup_reflect_value").Register("setup_reflect_value", before)
	_ = db.Callback().Row().Before("gorm:row").Register("row", before)
	_ = db.Callback().Raw().Before("gorm:raw").Register("raw", before)
	// end
	_ = db.Callback().Create().After("gorm:after_create").Register("after_create", afterTraceHandler)
	_ = db.Callback().Query().After("gorm:after_query").Register("after_query", afterTraceHandler)
	_ = db.Callback().Delete().After("gorm:after_delete").Register("after_delete", afterTraceHandler)
	_ = db.Callback().Update().After("gorm:after_update").Register("after_update", afterTraceHandler)
	_ = db.Callback().Row().After("gorm:row").Register("row_handler", afterTraceHandler)
	_ = db.Callback().Raw().After("gorm:raw").Register("raw_handler", afterTraceHandler)
	return nil
}

func afterTraceHandler(db *gorm.DB) {
	ctx := db.Statement.Context
	gCtx, ok := ctx.(*gin.Context)
	if !ok {
		return
	}

	trace, ok := GetGinTraceCtx(gCtx)
	if !ok {
		return
	}

	_ts, ok := db.InstanceGet("startTime")
	if !ok {
		return
	}

	ts := _ts.(time.Time)
	sqlStr := db.Dialector.Explain(db.Statement.SQL.String(), db.Statement.Vars...)
	var stack string
	link, ok := db.Get("TraceLineNum")
	if ok {
		stack = link.(string)
	}

	trace.AppendSQL(&LinkTraceSQL{
		Timestamp: GetFullTime(ts.Unix()),
		Stack:     stack,
		SQL:       sqlStr,
		Rows:      db.Statement.RowsAffected,
		Cost:      time.Since(ts).String(),
	})
}

func HandleGormQueryErrorFromTx(tx *gorm.DB) (*gorm.DB, error) {
	return tx, HandleGormQueryError(tx.Error)
}

func HandleGormQueryError(err error) error {
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	return err
}
