package fit

import (
	"context"
	"encoding/json"
	"errors"
	"go.etcd.io/etcd/client/v3"
	"sync"
	"time"

	"google.golang.org/grpc/resolver"
)

type ThisServiceType string
type ThisServiceStatus int

const (
	ServiceTypeBasic ThisServiceType = "BASIC"
	ServiceTypeClone ThisServiceType = "CLONE"
)

const (
	// ServiceStatusRun In operation
	ServiceStatusRun ThisServiceStatus = iota

	// ServiceStatusWaitDone Waiting for the resource connection in the service to be released,
	// at which point the service refuses to provide external services
	ServiceStatusWaitDone
)

type RegisterCenterValue struct {
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
	//Service address, HTTP service plus full protocol.
	Addr string `json:"addr"`

	//Is the service available? If not, then traffic should not be forwarded to this service again.
	//The reason for the unavailability can be analyzed through the Reason field
	Available bool   `json:"available"`
	Reason    string `json:"reason"`

	//Service types are divided into basic services and clone services.
	//Basic services are manually initiated, so the monitoring system should not delete these services.
	//Clone services are dynamically cloned by the monitoring system (or other systems) based on the current machine load situation,
	//and these services are not stable
	EntityType ThisServiceType `json:"entity_type"`

	// Service status
	Status ThisServiceStatus `json:"status"`
}

func NewRegisterCenterValue(addr string) string {
	return H{
		"created_at":  time.Now().Unix(),
		"updated_at":  time.Now().Unix(),
		"addr":        addr,
		"available":   true,
		"entity_type": ServiceTypeBasic,
		"status":      ServiceStatusRun,
	}.ToString()
}

func NewRegistrationCenterValueOption(option RegisterCenterValue) string {
	result, err := json.Marshal(&option)
	if err != nil {
		return ""
	}
	return string(result)
}

func IsHeightLoad(val string) bool {
	if val == "_HEIGHT_LOAD" {
		return true
	}
	return false
}

func IsWaitDone(val string) bool {
	if val == "_AWAIT_DONE" {
		return true
	}
	return false
}

type Resolver struct {
	sync.RWMutex
	Client *clientv3.Client
	cc     resolver.ClientConn
	prefix string
}

func (r *Resolver) ResolveNow(resolver.ResolveNowOptions) {
	// TODO:watch It will be called after changes
}

func (r *Resolver) Close() {
	// TODO: Parser off
}

func (r *Resolver) watcher() {
	response, err := r.Client.Get(context.Background(), r.prefix, clientv3.WithPrefix())
	if err != nil {
		return
	}
	addresses := make([]resolver.Address, 0)
	for _, kv := range response.Kvs {
		var rg RegisterCenterValue
		if err := json.Unmarshal(kv.Value, &rg); err != nil {
			break
		}
		addresses = append(addresses, resolver.Address{ServerName: string(kv.Key), Addr: rg.Addr})
	}
	if len(addresses) == 0 {
		r.cc.ReportError(errors.New("no available services"))
		return
	}

	r.cc.UpdateState(resolver.State{
		Addresses: addresses,
	})

	//TODO: Listening has been canceled here
}
