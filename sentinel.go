package fit

import (
	"errors"
	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/core/flow"
)

type SentinelConfig struct {
	Version string
	AppName string
	LogDir  string
}

// InitSentinel necessary
// After initialization, you need to call the loadflowrule and loadcreaterrule
// functions to load the rules
func InitSentinel(cf SentinelConfig) error {
	conf := config.NewDefaultConfig()
	if cf.LogDir != "" {
		conf.Sentinel.Log.Dir = cf.LogDir
	}
	conf.Version = cf.Version
	conf.Sentinel.App.Name = cf.AppName
	err := sentinel.InitWithConfig(conf)
	if err != nil {
		Error("sentinel init error", "sentinel init error", err)
		return err
	}
	return nil
}

// LoadFlowRule Load create restrictor rule
func LoadFlowRule(rule []*flow.Rule) error {
	_, err := flow.LoadRules(rule)
	if err != nil {
		return err
	}
	return nil
}

// LoadBreakerRule Load fuse rule
func LoadBreakerRule(rule []*circuitbreaker.Rule, inf ...circuitbreaker.StateChangeListener) error {
	if len(inf) > 0 {
		circuitbreaker.RegisterStateChangeListeners(inf[0])
	}
	_, err := circuitbreaker.LoadRules(rule)
	if err != nil {
		return err
	}
	return nil
}

//Entry is the basic API of Sentinel
//
// Please call NewDefaultBuilder or NewBuilder before calling this function
func Entry(ruleName string, fn func() error) error {
	e, b := sentinel.Entry(ruleName)
	if b != nil {
		return errors.New("operation failed. The failure reason may be external service error")
	} else {
		err := fn()
		if err != nil {
			sentinel.TraceError(e, err)
			e.Exit()
			return err
		}
		e.Exit()
		return nil
	}
}
