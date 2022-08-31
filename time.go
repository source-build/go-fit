package fit

import (
	"github.com/golang-module/carbon"
	"time"
)

// BeforeDawnTimeDifference 此刻到明日凌晨00：00的时间差
func BeforeDawnTimeDifference() time.Duration {
	now := time.Now()
	next := now.Add(time.Hour * 24)
	next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
	return next.Sub(now)
}

// SpecifiedTimeExceeded 当前是否超过了给定时间
func SpecifiedTimeExceeded(unix int64) bool {
	if time.Now().Unix()-unix < 0 {
		return false
	}
	return true
}

// GetFullTime return format: yy-mm-dd h:m:s
func GetFullTime(unix int64) string {
	return time.Unix(unix, 0).Format("2006-01-02 15:04:05")
}

// GetTimeStr return format: yy-mm-dd h:m:s
func GetTimeStr(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// GetHMS  h:m:s
func GetHMS(unix int64) string {
	return time.Unix(unix, 0).Format("15:04:05")
}

// GetMS  h:s
func GetMS(unix int64) string {
	return time.Unix(unix, 0).Format("15:04")
}

func UnixToTime(unix int64) carbon.Carbon {
	return carbon.CreateFromTimestamp(unix)
}
