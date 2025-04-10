package fit

import (
	"time"
)

// BeforeDawnTimeDifference Time difference between now and 00:00 am tomorrow
func BeforeDawnTimeDifference() time.Duration {
	now := time.Now()
	next := now.Add(time.Hour * 24)
	next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
	return next.Sub(now)
}

// SpecifiedTimeExceeded Whether the current time exceeds the given time
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
