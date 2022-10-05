package fit

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"time"
)

var mysqlClient *gorm.DB

type DefaultConfigMysql struct {
	User string
	Pass string
	IP   string
	Port string
	DB   string
}

func NewMysqlDefConnect(config DefaultConfigMysql, useTrace bool) error {
	dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?charset=utf8mb4&parseTime=True&loc=Local", config.User, config.Pass, config.IP, config.Port, config.DB)
	client, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	if useTrace {
		err = client.Use(new(TracePlugin))
		if err != nil {
			return err
		}
	}

	mysqlClient = client

	// connection pool,use default config
	sqlDb, err := client.DB()
	sqlDb.SetMaxIdleConns(10)
	sqlDb.SetMaxOpenConns(200)
	sqlDb.SetConnMaxLifetime(time.Hour)
	return nil
}

// NewMysqlConnect  init new mysql client
// param: addr mysql address, format: root:123@tcp(127.0.0.1:3369)/foo?charset=utf8mb4&parseTime=True&loc=Local
// param: config mysql config
// param: isUsePool use connection pool
func NewMysqlConnect(addr string, config *gorm.Config, usePool, useTrace bool) (*sql.DB, error) {
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

	// connection pool
	var pool *sql.DB
	if usePool {
		pool, err = client.DB()
		if err != nil {
			return pool, err
		}
	}

	return pool, nil
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
