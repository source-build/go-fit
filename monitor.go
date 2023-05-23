package fit

import (
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go/v4"
	"github.com/denisbrodbeck/machineid"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/net/context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	pushTypeIsNil  = ""
	pushTypeIsMQ   = "MQ"
	pushTypeIsHttp = "HTTP"

	INIT_MODE = "INIT"
	WORK_MODE = "WORK"
)

type mqRemoteConfig struct {
	mqDeclareName    string
	mqDeclareDurable bool
	mqAutoDelete     bool

	mqExchangeName    string
	mqExchangeDurable bool
	mqRoutingKey      string
}

type IoCounter struct {
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
}

type HostInfo struct {
	Hostname        string `json:"hostname"`
	HostId          string `json:"host_id"`
	PlatformVersion string `json:"platform_version"`
	KernelArch      string `json:"kernel_arch"`
	Procs           uint64 `json:"procs"`
}

type VirtualMemory struct {
	Total       uint64
	Available   uint64
	Used        uint64
	UsedPercent float64
}

type RedisInfo struct {
	RedisInfoClients string `json:"redis_info_clients"`
	RedisInfoStats   string `json:"redis_info_stats"`
	RedisInfoMemory  string `json:"redis_info_memory"`
}

type MessageBody struct {
	Stage     string `json:"stage"`
	WorkTasks int32  `json:"work_tasks"`
	//format: type/name/node
	Name string `json:"name"`
	//format: ip:port
	Address       string  `json:"address"`
	CpuPercent    float64 `json:"cpu_percent"`
	SystemVersion string  `json:"system_version"`
	PhysicalId    string  `json:"physical_id"`
	HostInfo
	IoCounter
	VirtualMemory
	RedisInfo
	Time time.Time `json:"time"`
}

type ServiceMonitorOption struct {
	Context               context.Context
	ServiceNode           string
	ServiceName           string
	ServiceType           string
	ServiceAddress        string
	SystemVersion         string
	RecordRedisClientInfo bool
	RecordRedisStatsInfo  bool
	RecordRedisMemoryInfo bool
}

type monitorTask struct {
	option       *ServiceMonitorOption
	etcdv3       *EtcdHandle
	isActive     bool
	atWork       bool
	quitLoopChan chan bool
	uniqueCode   string
	isDown       bool
}

var onlineGrouting int32

func OnlineGroutingAdd() {
	onlineGrouting++
}

func OnlineGroutingCut() {
	onlineGrouting--
}

func ServiceMonitorTask(option *ServiceMonitorOption) error {
	etcdv3, err := MainEtcdv3(option.Context)
	if err != nil {
		return err
	}

	config, err := etcdv3.Get("/service/config")
	if err != nil {
		return err
	}

	result, err := ExtractValAndToMap(config)
	if err != nil {
		return err
	}

	pfx, ok := result["serviceMonitoringPfx"].(string)
	if !ok {
		return NewErr("serviceMonitoringPfx assertion failed")
	}

	taskData := monitorTask{
		option: option,
		etcdv3: etcdv3,
	}

	go func(pfx string, task *monitorTask) {
		pfx = StringSpliceTag("/", pfx, task.option.ServiceType, task.option.ServiceName)
		fmt.Println(pfx)

		rch := MainEtcdClientv3().Watch(task.option.Context, pfx, clientv3.WithPrefix())
		for wresp := range rch {
			for _, ev := range wresp.Events {
				switch ev.Type {
				case mvccpb.PUT:
					if err := task.watchPutHandler(ev.Kv.Key, ev.Kv.Value); err != nil {
						Error("msg", "service monitoring Watch Put failed", "err", err)
					}
				case mvccpb.DELETE:
					task.close()
				}
			}
		}
	}(pfx, &taskData)
	return nil
}

func (m *monitorTask) watchPutHandler(key, value []byte) error {
	data := make(map[string]any, 0)
	if err := json.Unmarshal(value, &data); err != nil {
		return err
	}

	//This service is responsible for collecting
	keyArr := strings.Split(string(key), "/")
	if keyArr[len(keyArr)-1] == m.option.ServiceNode {
		m.isActive = true
	}

	return m.taskController(data)
}

func (m *monitorTask) taskController(obj map[string]any) error {
	stage, ok := obj["stage"].(string)
	if !ok || stage == "" {
		return NewErr("stage cannot be empty")
	}

	if err := checkParams(obj); err != nil {
		return err
	}

	if stage == INIT_MODE {
		m.quitLoopChan = make(chan bool, 1)
		m.uniqueCode = GetMachineCode()

		name := StringSpliceTag("/", m.option.ServiceType, m.option.ServiceName, m.option.ServiceNode)
		body := MessageBody{
			Stage:         INIT_MODE,
			Name:          name,
			Address:       m.option.ServiceAddress,
			SystemVersion: m.option.SystemVersion,
			PhysicalId:    m.uniqueCode,
			Time:          time.Now(),
		}
		if hostInfo, err := host.Info(); err == nil {
			body.Hostname = hostInfo.Hostname
			body.HostId = hostInfo.HostID
			body.KernelArch = hostInfo.KernelArch
			body.PlatformVersion = hostInfo.PlatformVersion
		}
		subType := obj["subType"].(string)
		if subType == pushTypeIsMQ {
			mq, err := NewRabbitMQ()
			if err != nil {
				return err
			}
			defer mq.Close()
			config := m.extractMqInfo(obj)
			err = m.sendMqMessage(mq, obj, &body, config)
			return err
		}
		if subType == pushTypeIsHttp {
			return m.sendHttpMessage(obj, &body)
		}
		return nil
	}

	if stage == WORK_MODE {
		if m.atWork {
			return nil
		}
		m.quitLoopChan = make(chan bool, 1)
		m.atWork = true
		m.uniqueCode = GetMachineCode()
		go m.continuousWork(obj)
	}

	return nil
}

func (m *monitorTask) close() {
	if m.quitLoopChan == nil {
		return
	}
	m.quitLoopChan <- true
	close(m.quitLoopChan)
	m.isActive = false
	m.atWork = false
	m.isDown = false
	fmt.Println("关闭")
}

func (m *monitorTask) continuousWork(obj map[string]any) {
	durationSource, ok := obj["duration"].(int)
	if !ok {
		durationSource = 5
	}

	duration := time.Duration(durationSource)
	returnWorkTask, ok := obj["returnWorkTask"].(bool)
	if !ok {
		returnWorkTask = false
	}

	returnMem, ok := obj["returnMem"].(bool)
	if !ok {
		returnMem = false
	}

	returnCpu, ok := obj["returnCpu"].(bool)
	if !ok {
		returnCpu = false
	}

	returnIoCount, ok := obj["returnIoCount"].(bool)
	if !ok {
		returnIoCount = false
	}

	retryCount, ok := obj["retryCount"].(uint)
	if !ok {
		retryCount = 5
	}

	var mq *RabbitMQ
	var err error
	subType := obj["subType"].(string)
	if subType == pushTypeIsMQ {
		mq, err = NewRabbitMQ()
		defer mq.Close()
	}
	if err != nil {
		Error("msg", "newRabbitMQ instance failed!", "err", err)
		return
	}

	config := m.extractMqInfo(obj)
	name := StringSpliceTag("/", m.option.ServiceType, m.option.ServiceName, m.option.ServiceNode)
	body := MessageBody{
		Stage:         WORK_MODE,
		Name:          name,
		Address:       m.option.ServiceAddress,
		SystemVersion: m.option.SystemVersion,
		PhysicalId:    m.uniqueCode,
		Time:          time.Now(),
	}
	if hostInfo, err := host.Info(); err == nil {
		body.Hostname = hostInfo.Hostname
		body.HostId = hostInfo.HostID
		body.KernelArch = hostInfo.KernelArch
		body.PlatformVersion = hostInfo.PlatformVersion
	}
	for {
		select {
		case <-m.quitLoopChan:
			m.quitLoopChan = nil
			return
		case <-m.option.Context.Done():
			return
		default:
		}

		if m.isDown {
			time.Sleep(time.Second * duration)
			continue
		}

		if returnWorkTask {
			body.WorkTasks = onlineGrouting
		}

		if err := PingEtcd(); err != nil {
			m.isDown = true
			go func(m *monitorTask) {
				err := retry.Do(func() error {
					if err := PingEtcd(); err != nil {
						return err
					}
					return nil
				}, retry.Attempts(retryCount), retry.OnRetry(func(n uint, err error) {
					Error("business", "service monitoring information collection node", "msg", "The etcd is unavailable, and the connection is being retried,The maximum number of retries is "+strconv.Itoa(int(retryCount)), "count", n+1)
				}))
				if err != nil {
					m.close()
					Error("business", "service monitoring information collection node", "msg", "The etcd is unavailable, the number of retries has reached the maximum, and the task has been closed!", "err", err)
					return
				}
				m.isDown = false
				Error("business", "service monitoring information collection node", "The etcd is unavailable, and the connection is retried successfully. The task has been recovered", "err", err)
			}(m)
			continue
		}

		body.Time = time.Now()

		if m.isActive {
			if hostInfo, err := host.Info(); err == nil {
				body.Procs = hostInfo.Procs
			}

			if returnMem {
				body.VirtualMemory = GetVirtualMemory()
			}

			if returnCpu {
				totalPercent, _ := cpu.Percent(time.Second, false)
				if len(totalPercent) > 0 {
					body.CpuPercent = totalPercent[0]
				}
			}

			if returnIoCount {
				body.IoCounter = GetIOCounters()
			}
		}

		redisInfo, err := CollectRedisInfo(m.option.RecordRedisClientInfo, m.option.RecordRedisStatsInfo, m.option.RecordRedisMemoryInfo)
		if err == nil {
			if m.option.RecordRedisClientInfo {
				body.RedisInfo.RedisInfoClients = redisInfo.RedisInfoClients
			}
			if m.option.RecordRedisStatsInfo {
				body.RedisInfo.RedisInfoStats = redisInfo.RedisInfoStats
			}
			if m.option.RecordRedisMemoryInfo {
				body.RedisInfo.RedisInfoMemory = redisInfo.RedisInfoMemory
			}
		}

		if subType == pushTypeIsMQ {
			if err := m.sendMqMessage(mq, obj, &body, config); err != nil {
				Error("business", "service monitoring information collection node", "msg", "mq send failed!!", "err", err)
			}
		}

		if subType == pushTypeIsHttp {
			if err := m.sendHttpMessage(obj, &body); err != nil {
				Error("business", "service monitoring information collection node", "msg", "http request failed!!", "err", err)
			}
		}

		time.Sleep(time.Second * duration)
	}
}

func (m *monitorTask) extractMqInfo(obj map[string]any) *mqRemoteConfig {
	var mqConfig mqRemoteConfig
	mqAutoDelete, ok := obj["mqAutoDelete"].(bool)
	if !ok {
		mqAutoDelete = true
	}
	mqConfig.mqAutoDelete = mqAutoDelete

	mqWorkType := obj["mqWorkType"].(string)
	if mqWorkType == "simple" || mqWorkType == "work" {
		mqDeclareName, ok := obj["mqDeclareName"].(string)
		if !ok {
			mqDeclareName = ""
		}
		mqDeclareDurable, ok := obj["mqDeclareDurable"].(bool)
		if !ok {
			mqDeclareDurable = false
		}
		mqConfig.mqDeclareName = mqDeclareName
		mqConfig.mqDeclareDurable = mqDeclareDurable
	}
	if mqWorkType == "publish" {
		mqExchangeName, ok := obj["mqExchangeName"].(string)
		if !ok {
			mqExchangeName = ""
		}
		mqExchangeDurable, ok := obj["mqExchangeDurable"].(bool)
		if !ok {
			mqExchangeDurable = false
		}
		mqConfig.mqExchangeName = mqExchangeName
		mqConfig.mqExchangeDurable = mqExchangeDurable
	}
	if mqWorkType == "routing" {
		mqExchangeName, ok := obj["mqExchangeName"].(string)
		if !ok {
			mqExchangeName = ""
		}
		mqExchangeDurable, ok := obj["mqExchangeDurable"].(bool)
		if !ok {
			mqExchangeDurable = false
		}
		mqRoutingKey, ok := obj["mqRoutingKey"].(string)
		if !ok {
			mqRoutingKey = "monitor"
		}
		mqConfig.mqExchangeName = mqExchangeName
		mqConfig.mqExchangeDurable = mqExchangeDurable
		mqConfig.mqRoutingKey = mqRoutingKey
	}
	return &mqConfig
}

func (m *monitorTask) sendMqMessage(mq *RabbitMQ, obj map[string]any, body *MessageBody, mqConfig *mqRemoteConfig) error {
	mqWorkType := obj["mqWorkType"].(string)
	if mqWorkType == "simple" || mqWorkType == "work" {
		bodyMsg, err := json.Marshal(body)
		if err != nil {
			return err
		}
		return mq.DefQueueDeclare(mqConfig.mqDeclareName, mqConfig.mqDeclareDurable, mqConfig.mqAutoDelete).PublishSimple(string(bodyMsg))
	}
	if mqWorkType == "publish" {
		bodyMsg, err := json.Marshal(body)
		if err != nil {
			return err
		}
		return mq.ExchangeDeclare(mqConfig.mqExchangeName, KIND_FANOUT, mqConfig.mqExchangeDurable, mqConfig.mqAutoDelete, false, false, nil).PublishPub(string(bodyMsg))
	}
	if mqWorkType == "routing" {
		bodyMsg, err := json.Marshal(body)
		if err != nil {
			return err
		}
		return mq.ExchangeDeclare(mqConfig.mqExchangeName, KIND_DIRECT, mqConfig.mqExchangeDurable, mqConfig.mqAutoDelete, false, false, nil).Publish(string(bodyMsg), mqConfig.mqRoutingKey)
	}
	return nil
}

func (m *monitorTask) sendHttpMessage(obj map[string]any, body *MessageBody) error {
	url, ok := obj["subHttpUrl"].(string)
	if !ok {
		return NewErr("SubHttpUrl cannot be empty！")
	}
	h := H{}
	if subHttpToken, ok := obj["subHttpToken"].(string); ok {
		h["Authorization"] = "Bearer " + subHttpToken
	}

	subHttpHeader, ok := obj["subHttpHeader"].(H)
	if !ok {
		subHttpHeader = H{}
	} else {
		h = subHttpHeader
	}

	bodyStr, err := json.Marshal(body)
	if err != nil {
		return err
	}

	return retry.Do(func() error {
		resObj := new(HttpUtil).NewRequest(http.MethodPost, url, string(bodyStr))
		if resObj.Err != nil {
			return resObj.Err
		}

		response, err := resObj.Response()
		if err != nil {
			return err
		}

		if response.StatusCode != http.StatusOK {
			return NewErr("request failed!")
		}

		return nil
	}, retry.Attempts(3))
}

func checkParams(obj map[string]any) error {
	stage, ok := obj["stage"].(string)
	if !ok || stage == "" {
		return NewErr("stage cannot be empty")
	}

	subType, ok := obj["subType"].(string)
	if !ok || subType == pushTypeIsNil {
		return NewErr("subType cannot be empty")
	}

	if subType == pushTypeIsMQ {
		if mqWorkType, ok := obj["mqWorkType"].(string); !ok || mqWorkType == "" {
			return NewErr("mqWorkType cannot be empty")
		}
	}

	if subType == pushTypeIsHttp {
		if subHttpUrl, ok := obj["subHttpUrl"].(string); !ok || subHttpUrl == "" {
			return NewErr("subHttpUrl cannot be empty")
		}
	}
	return nil
}

func GetMachineCode(myApp ...string) string {
	var id string
	var err error
	if len(myApp) > 0 {
		id, err = machineid.ProtectedID(myApp[0])
	} else {
		id, err = machineid.ID()
	}
	if err != nil {
		return ""
	}
	return id
}

func GetIOCounters() (io IoCounter) {
	n2, err := net.IOCounters(false)
	if err != nil {
		return io
	}
	if len(n2) > 0 {
		io.BytesSent = n2[0].BytesSent
		io.BytesRecv = n2[0].BytesRecv
		io.PacketsSent = n2[0].PacketsSent
		io.PacketsRecv = n2[0].PacketsRecv
	}
	return io
}

func GetVirtualMemory() (memInfo VirtualMemory) {
	m, err := mem.VirtualMemory()
	if err != nil {
		return memInfo
	}
	memInfo.Total = m.Total
	memInfo.Available = m.Available
	memInfo.Used = m.Used
	memInfo.UsedPercent = m.UsedPercent
	return memInfo
}

func CollectRedisInfo(returnClient, returnStats, returnMemory bool) (*RedisInfo, error) {
	result, err := MainRedis().GetNode().Info(context.Background()).Result()
	if err != nil {
		return nil, err
	}
	strArr := strings.Split(result, "\n\r")

	if len(strArr) == 0 {
		return nil, NewErr("get redis info failed!")
	}

	if !returnClient && !returnStats && !returnMemory {
		return nil, NewErr("at least one is true!")
	}

	var info RedisInfo

	if returnClient {
		info.RedisInfoClients = strArr[1]
	}

	if returnStats {
		info.RedisInfoStats = strArr[5]
	}

	if returnMemory {
		info.RedisInfoMemory = strArr[3]
	}

	return &info, nil
}
