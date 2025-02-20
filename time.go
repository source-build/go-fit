package fit

import (
	"encoding/json"
	"time"
)

// Time Suitable for time types in the format "yyyy-mm-dd_hh-mm-ss."
type Time time.Time

func (c *Time) Time() time.Time {
	return time.Time(*c)
}

func (c *Time) StdTime(t time.Time) {
	*c = Time(t)
}

func (c *Time) MarshalJSON() ([]byte, error) {
	t := time.Time(*c)
	return json.Marshal(t.Format("2006-01-02 15:04:05"))
}

func (c *Time) UnmarshalJSON(b []byte) error {
	t, err := time.Parse(`"2006-01-02 15:04:05"`, string(b))
	if err != nil {
		return err
	}
	*c = Time(t)
	return nil
}

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
