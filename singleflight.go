package fit

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sync/singleflight"
)

type Single struct {
	Forget time.Duration
}

func NewSingle(t ...time.Duration) *Single {
	var d time.Duration
	if len(t) > 0 {
		d = t[0]
	}
	return &Single{Forget: d}
}

func (s *Single) DoChan(ctx context.Context, sg *singleflight.Group, key string, fn func() (interface{}, error)) (v interface{}, err error, shared bool) {
	if s.Forget > 0 {
		time.Sleep(s.Forget)
		sg.Forget(key)
	}
	result := sg.DoChan(key, fn)
	select {
	case r := <-result:
		return r.Val, r.Err, r.Shared
	case <-ctx.Done():
		return nil, errors.New("singleflight DoChan timeout"), false
	}
}
