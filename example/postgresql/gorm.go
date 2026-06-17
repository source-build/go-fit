package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/source-build/go-fit"
	"github.com/source-build/go-fit/flog"
	"gorm.io/gorm"
)

// User 覆盖 PostgreSQL 常用数据类型，每个字段通过 gorm tag 指定 PG 类型。
type User struct {
	fit.Model

	// ===== 数值类型 =====
	Smallint        int16   `gorm:"type:smallint;comment:小范围整数,2字节(-32768~32767)"`
	Integer         int32   `gorm:"type:integer;comment:普通整数,4字节"`
	Bigint          int64   `gorm:"type:bigint;comment:大范围整数,8字节"`
	Decimal         float64 `gorm:"type:decimal(12,2);comment:定点数,精度12标度2"`
	Numeric         float64 `gorm:"type:numeric(14,4);comment:任意精度数值,精度14标度4"`
	Real            float32 `gorm:"type:real;comment:单精度浮点,4字节"`
	DoublePrecision float64 `gorm:"type:double precision;comment:双精度浮点,8字节"`
	Money           string  `gorm:"type:money;comment:货币金额,固定两位小数(读出来是带$和千分位的本地化字符串)"`

	// ===== 字符类型 =====
	Varchar   string `gorm:"type:varchar(50);comment:变长字符串,最长50"`
	CharFixed string `gorm:"type:char(10);comment:定长字符串,不足补空格"`
	TextVal   string `gorm:"type:text;comment:任意长度文本"`

	// ===== 二进制 =====
	Bytea []byte `gorm:"type:bytea;comment:二进制数据(字节数组)"`

	// ===== 日期/时间类型 =====
	DateCol     time.Time `gorm:"type:date;comment:日期(仅年月日)"`
	TimeCol     time.Time `gorm:"type:time;comment:时间(无时区)"`
	Timetz      time.Time `gorm:"type:timetz;comment:时间(带时区)"`
	Timestamp   time.Time `gorm:"type:timestamp;comment:时间戳(无时区)"`
	Timestamptz time.Time `gorm:"type:timestamptz;comment:时间戳(带时区)"`
	IntervalCol string    `gorm:"type:interval;comment:时间间隔"`

	// ===== 布尔 =====
	BoolVal bool `gorm:"type:boolean;comment:布尔值(true/false)"`

	// ===== JSON =====
	JSONCol  string `gorm:"type:json;comment:JSON文本"`
	JSONBCol string `gorm:"type:jsonb;comment:JSON二进制,可索引"`

	// ===== UUID =====
	UUIDCol string `gorm:"type:uuid;comment:UUID主键/标识"`

	// ===== 位串 =====
	BitCol    string `gorm:"type:bit(8);comment:定长位串,8位"`
	VarbitCol string `gorm:"type:varbit(16);comment:变长位串,最长16位"`

	// ===== 网络地址 =====
	CidrCol string `gorm:"type:cidr;comment:IPv4/IPv6网络(含掩码)"`
	InetCol string `gorm:"type:inet;comment:IPv4/IPv6主机地址"`
	Macaddr string `gorm:"type:macaddr;comment:MAC地址"`

	// ===== XML =====
	XMLCol string `gorm:"type:xml;comment:XML数据"`

	// ===== 数组 =====
	TextArray string `gorm:"type:text[];comment:文本数组"`
	Int4Array string `gorm:"type:int[];comment:整数数组"`

	// ===== 几何类型(PG特有,示例 point) =====
	PointCol string `gorm:"type:point;comment:几何点(x,y)"`
}

func main() {
	opt := flog.Options{
		LogLevel:          flog.ErrorLevel,
		EncoderConfigType: flog.ProductionEncoderConfig,
		Console:           false,
		// 默认文件输出，为空表示不输出到文件
		Filename: "logs/postgresql.log",
	}
	gormLogger := flog.NewGormLogger(opt)

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

	c, err := fit.NewPostgreSQLDefaultClient(fit.PostgreSQLClientOption{
		Username: "postgres",
		Password: "123456",
		Host:     "localhost",
		Port:     "5432",
		DbName:   "postgres",
		Config:   gormConfig,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer c()

	// 每次运行先删后建，保证表结构与当前结构体完全一致（demo 用）
	if err := fit.PG.Migrator().DropTable(&User{}); err != nil {
		log.Println("drop table:", err)
	}
	if err := fit.PG.AutoMigrate(&User{}); err != nil {
		log.Fatal("auto migrate failed: ", err)
	}

	// 打印「Go 字段 -> PG 实际类型」映射
	printColumnTypes()

	u := randomUser()
	if err := fit.PG.Create(&u).Error; err != nil {
		log.Fatal("create failed: ", err)
	}
	fmt.Printf("inserted user id=%d\n", u.ID)
}

// printColumnTypes 查询 information_schema，打印 users 表每列的 PG 数据类型。
func printColumnTypes() {
	type column struct {
		ColumnName string
		DataType   string
		UdtName    string
	}
	var cols []column
	err := fit.PG.Table("information_schema.columns").
		Select("column_name", "data_type", "udt_name").
		Where("table_name = ?", "users").
		Order("ordinal_position").
		Scan(&cols).Error
	if err != nil {
		fmt.Println("query column types failed:", err)
		return
	}

	fmt.Println("\n===== users 表列类型映射 (column -> data_type / udt_name) =====")
	fmt.Printf("%-20s %-35s %s\n", "column", "data_type", "udt_name")
	for _, c := range cols {
		fmt.Printf("%-20s %-35s %s\n", c.ColumnName, c.DataType, c.UdtName)
	}
	fmt.Println("=================================================================\n")
}

// randomUser 生成一条各字段均为合法随机值的记录。
func randomUser() User {
	now := time.Now()
	return User{
		Smallint:        int16(rand.Intn(32767)),
		Integer:         rand.Int31(),
		Bigint:          rand.Int63(),
		Decimal:         float64(rand.Intn(1000000)) + rand.Float64(),
		Numeric:         float64(rand.Intn(1000000)) + rand.Float64(),
		Real:            rand.Float32(),
		DoublePrecision: rand.Float64(),
		Money:           fmt.Sprintf("%d.%02d", rand.Intn(100000), rand.Intn(100)),
		Varchar:         randomString(8),
		CharFixed:       randomString(10),
		TextVal:         "随机文本-" + randomString(12),
		Bytea:           []byte(randomString(16)),
		DateCol:         now,
		TimeCol:         now,
		Timetz:          now,
		Timestamp:       now,
		Timestamptz:     now,
		IntervalCol:     fmt.Sprintf("%d days %02d:%02d:%02d", rand.Intn(30), rand.Intn(24), rand.Intn(60), rand.Intn(60)),
		BoolVal:         rand.Intn(2) == 1,
		JSONCol:         fmt.Sprintf(`{"name":"%s","age":%d,"active":%v}`, randomString(4), rand.Intn(80), rand.Intn(2) == 1),
		JSONBCol:        fmt.Sprintf(`{"k":"%s","v":%d}`, randomString(3), rand.Intn(999)),
		UUIDCol:         randomUUID(),
		BitCol:          randomBits(8),
		VarbitCol:       randomBits(16),
		CidrCol:         fmt.Sprintf("192.168.%d.0/24", rand.Intn(255)),
		InetCol:         fmt.Sprintf("10.0.%d.%d", rand.Intn(255), rand.Intn(255)),
		Macaddr:         fmt.Sprintf("08:00:2b:%02x:%02x:%02x", rand.Intn(255), rand.Intn(255), rand.Intn(255)),
		XMLCol:          fmt.Sprintf(`<user name="%s"/>`, randomString(5)),
		TextArray:       fmt.Sprintf("{%s,%s,%s}", randomString(3), randomString(3), randomString(3)),
		Int4Array:       fmt.Sprintf("{%d,%d,%d}", rand.Intn(100), rand.Intn(100), rand.Intn(100)),
		PointCol:        fmt.Sprintf("(%d,%d)", rand.Intn(100), rand.Intn(100)),
	}
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func randomBits(n int) string {
	b := make([]byte, n)
	for i := range b {
		if rand.Intn(2) == 1 {
			b[i] = '1'
		} else {
			b[i] = '0'
		}
	}
	return string(b)
}

// randomUUID 生成符合 PG uuid 格式(8-4-4-4-12)的随机十六进制字符串。
func randomUUID() string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		rand.Uint32(),
		rand.Uint32()&0xFFFF,
		rand.Uint32()&0xFFFF,
		rand.Uint32()&0xFFFF,
		rand.Uint64()&0xFFFFFFFFFFFF,
	)
}
