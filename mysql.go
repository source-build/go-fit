package fit

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Time Applicable to gorm time types in the format of 'yyyy-mm-dd_hh-mm-ss'
type Time time.Time

func (t *Time) PtrTime() *Time {
	return t
}

func (t *Time) MarshalJSON() ([]byte, error) {
	tlt := time.Time(*t)
	return json.Marshal(tlt.Format("2006-01-02 15:04:05"))
}

func (t *Time) UnmarshalJSON(b []byte) error {
	tlt, err := time.Parse(`"2006-01-02 15:04:05"`, string(b))
	if err != nil {
		return err
	}
	*t = Time(tlt)
	return nil
}

func (t *Time) Value() (driver.Value, error) {
	var zeroTime time.Time
	if t == nil {
		return nil, nil
	}

	tlt := time.Time(*t)
	if tlt.UnixNano() == zeroTime.UnixNano() {
		return nil, nil
	}

	return tlt, nil
}

func (t *Time) Scan(v interface{}) error {
	if value, ok := v.(time.Time); ok {
		*t = Time(value)
		return nil
	}

	return fmt.Errorf("can not convert %v to timestamp", v)
}

func TimePtr(t time.Time) *Time {
	tlt := Time(t)
	return &tlt
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

// CloseMySQLClient Close MySQL client
func CloseMySQLClient() error {
	sqlDb, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDb.Close()
}

func InjectMySQLClient(db *gorm.DB) {
	DB = db
}

// Model Encapsulated Gorm Model, Mainly added JSON format,
// if there is no such requirement, you can directly use Gorm Model
type Model struct {
	ID        uint           `gorm:"primarykey" json:"id,omitempty"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
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
