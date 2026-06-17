package fit

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostgreSQLClientOption struct {
	Username string

	Password string

	// Database server host
	Host string

	// default "5432"
	Port string

	// Can be empty
	DbName string

	// default："sslmode=disable&TimeZone=Asia/Shanghai"
	Params url.Values

	// Do not use connection pool
	DisableConnPool bool

	// Set the maximum number of idle connections
	MaxIdleConns int

	// Set the maximum number of open connections
	MaxOpenConns int

	// Set the maximum lifetime of the connection
	ConnMaxLifetime time.Duration

	// Set the maximum idle time of the connection
	ConnMaxIdleTime time.Duration

	Config *gorm.Config
}

// PG PostgreSQL client
var PG *gorm.DB

// NewPostgreSQLDefaultClient Create a quick PostgreSQL client that only requires commonly used configurations
// and returns a close function. The caller invokes the returned function to release the connection.
// For custom configurations, please use InjectPostgreSQLClient
func NewPostgreSQLDefaultClient(opt PostgreSQLClientOption) (func() error, error) {
	if opt.Username == "" || opt.Password == "" || opt.Host == "" {
		panic("Missing necessary parameters when initializing PostgreSQL")
	}

	if opt.Port == "" {
		opt.Port = "5432"
	}

	if opt.Params == nil {
		opt.Params = url.Values{}
		opt.Params.Set("sslmode", "disable")
		opt.Params.Set("TimeZone", "Asia/Shanghai")
	}

	config := opt.Config
	if opt.Config == nil {
		config = &gorm.Config{}
	}

	query := strings.ReplaceAll(opt.Params.Encode(), "%2F", "/")

	var err error
	PG, err = gorm.Open(
		postgres.Open(fmt.Sprintf("postgres://%s:%s@%s:%s/%s?%s", opt.Username, opt.Password, opt.Host, opt.Port, opt.DbName, query)),
		config)
	if err != nil {
		return nil, err
	}

	sqlDB, err := PG.DB()
	if err != nil {
		return nil, err
	}

	if !opt.DisableConnPool {
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

		if opt.ConnMaxIdleTime == 0 {
			opt.ConnMaxIdleTime = 30 * time.Minute
		}
		sqlDB.SetConnMaxIdleTime(opt.ConnMaxIdleTime)
	}

	return sqlDB.Close, nil
}

// ClosePostgreSQLClient Close PostgreSQL client
func ClosePostgreSQLClient() error {
	sqlDb, err := PG.DB()
	if err != nil {
		return err
	}
	return sqlDb.Close()
}

func InjectPostgreSQLClient(db *gorm.DB) {
	PG = db
}
