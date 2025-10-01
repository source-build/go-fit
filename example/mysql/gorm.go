package main

import (
	"log"
	"time"

	"github.com/source-build/go-fit"
	"github.com/source-build/go-fit/flog"
	"gorm.io/gorm"
)

type User struct {
	fit.Model
	UseTime *fit.Time `json:"use_time" gorm:"type:datetime"`
}

func main() {
	// 初始化一个日志
	opt := flog.Options{
		LogLevel:          flog.ErrorLevel,
		EncoderConfigType: flog.ProductionEncoderConfig,
		Console:           false,
		// 默认文件输出，为空表示不输出到文件
		Filename: "logs/mysql.log",
	}
	gormLogger := flog.NewGormLogger(opt)

	// 自定义日志配置
	gormConfig := &gorm.Config{
		SkipDefaultTransaction: false,
		NamingStrategy:         nil,
		FullSaveAssociations:   false,
		// 使用zap作为自定义日志
		// 自定义Logger，参考：https://github.com/go-gorm/gorm/blob/master/logger/logger.go
		Logger: fit.NewGormZapLogger(gormLogger, fit.GormZapLoggerOption{
			// 慢SQL阀值，默认200ms
			SlowThreshold: 500 * time.Millisecond,
			// 忽略 record not found 错误
			IgnoreRecordNotFoundError: true,
			// 禁用彩色输出
			DisableColorful: false,
		}),
		NowFunc:                                  nil,
		DryRun:                                   false,
		PrepareStmt:                              false,
		DisableAutomaticPing:                     false,
		DisableForeignKeyConstraintWhenMigrating: false,
		DisableNestedTransaction:                 false,
		AllowGlobalUpdate:                        false,
		QueryFields:                              false,
		CreateBatchSize:                          0,
		ClauseBuilders:                           nil,
		ConnPool:                                 nil,
		Dialector:                                nil,
		Plugins:                                  nil,
	}

	// 初始化mysql
	err := fit.NewMySQLDefaultClient(fit.MySQLClientOption{
		Username: "root",
		Password: "123456",
		Protocol: "tcp",
		Address:  "127.0.0.1:3306",
		DbName:   "user",
		// 自定义DSN参数，默认使用 charset=utf8&parseTime=True&loc=Local
		Params: nil,
		// 不使用连接池，默认启用
		DisableConnPool: false,
		// 设置空闲连接的最大数量，默认10
		MaxIdleConns: 0,
		// 设置打开连接的最大数量，默认100
		MaxOpenConns: 0,
		// 设置可以重复使用连接的最长时间，默认1h
		ConnMaxLifetime: 0,
		// gorm 配置
		Config: gormConfig,
	})
	if err != nil {
		log.Fatal(err)
	}

	//fit.InjectMySQLClient()
	//fit.DB
	//fit.DB.AutoMigrate(&User{})
}
