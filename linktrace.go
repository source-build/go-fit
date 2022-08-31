package fit

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// LinkTraceRequest request
type LinkTraceRequest struct {
	Method string      `json:"method"`
	Url    string      `json:"url"`
	Header interface{} `json:"header"`
	Body   interface{} `json:"body"`
}

// LinkTraceResponse response
type LinkTraceResponse struct {
	Header   interface{} `json:"header"`
	Body     interface{} `json:"body"`
	HttpCode int         `json:"http_code"`
	HttpMsg  string      `json:"http_msg"`
	Cost     string      `json:"cost"`
}

// LinkTraceDialog information of calling the third-party interface
type LinkTraceDialog struct {
	mux       sync.Mutex
	Request   *LinkTraceRequest    `json:"request"`
	Responses []*LinkTraceResponse `json:"responses"`
	Success   bool                 `json:"success"`
	Cost      string               `json:"cost"`
}

// LinkTraceSQL information about executing SQL
type LinkTraceSQL struct {
	Timestamp string `json:"timestamp"`     // format：2006-01-02 15:04:05
	Stack     string `json:"stack"`         // 文件地址和行号
	SQL       string `json:"sql"`           // SQL 语句
	Rows      int64  `json:"rows_affected"` // 影响行数
	Cost      string `json:"cost"`          // execution time
}

// LinkTraceRedis redis execution information
type LinkTraceRedis struct {
	Timestamp string      `json:"timestamp"` // format：2006-01-02 15:04:05
	Handle    string      `json:"handle"`    // operation，SET/GET...
	Args      interface{} `json:"args"`      // args
	Cost      string      `json:"cost"`      // execution time
}

// Trace recorded parameters
type Trace struct {
	mux                sync.Mutex
	TraceId            string             `json:"trace_id"`
	Request            *LinkTraceRequest  `json:"request"`
	Response           *LinkTraceResponse `json:"response"`
	ThirdPartyRequests []*LinkTraceDialog `json:"third_party_requests"`
	SQLs               []*LinkTraceSQL    `json:"sqls"`
	Redis              []*LinkTraceRedis  `json:"redis"`
	Success            bool               `json:"success"`
	Start              int64              `json:"start"`
	End                int64              `json:"end"`
	Cost               string             `json:"cost"`
	Extend             map[string]any     `json:"extend"`
}

func (t *Trace) AppendSQL(sqlInfo *LinkTraceSQL) {
	t.SQLs = append(t.SQLs, sqlInfo)
}

func (t *Trace) AppendRedis(row *LinkTraceRedis) {
	t.Redis = append(t.Redis, row)
}

func (t *Trace) Set(key string, value any) {
	if t.Extend == nil {
		t.Extend = make(map[string]any)
	}
	t.mux.Lock()
	defer t.mux.Unlock()
	t.Extend[key] = value
}

var trackCtxName string = "FIT_TRACE_CTX"

type responseWriter struct {
	gin.ResponseWriter
	b *bytes.Buffer
}

func (w responseWriter) Write(b []byte) (int, error) {
	w.b.Write(b)
	return w.ResponseWriter.Write(b)
}

func GetTraceCtxName() string {
	return trackCtxName
}

func TraceCaller(skip ...int) (key string, value string) {
	s := 1
	if len(skip) > 1 {
		s = skip[0]
	}
	_, file, line, ok := runtime.Caller(s)
	if !ok {
		return "", ""
	}
	_, fileName := filepath.Split(file)
	stack := StringSpliceTag(":", fileName, strconv.Itoa(line))
	return "TraceLineNum", stack
}

func ToTrace(source any) (trace *Trace, ok bool) {
	val, ok := source.(*Trace)
	if !ok {
		return nil, false
	}
	return val, true
}

func GetTraceCtx(c context.Context) (trace *Trace, ok bool) {
	val, ok := c.Value(trackCtxName).(*Trace)
	if !ok {
		return nil, false
	}
	return val, true
}

func GetGinTraceCtx(c *gin.Context) (trace *Trace, ok bool) {
	val, ok := c.Get(trackCtxName)
	if !ok {
		return nil, false
	}
	return val.(*Trace), true
}

type Hook interface {
	BeforeProcess(*Trace)
	AfterProcess(*Trace)
}

type GinTrace struct {
	LogFileName   string
	LogRecordMode string
	Func          func(*Trace)
	trace         *Trace
	hook          Hook
}

type TraceHandlerFunc func(*Trace)

// NewTrace create a new tracker.
func NewTrace(fileName ...string) *GinTrace {
	if len(fileName) > 0 {
		return &GinTrace{
			LogFileName: fileName[0],
		}
	}
	return &GinTrace{
		LogFileName: "trace",
	}
}

func (g *GinTrace) SetRecordMode(mode string) {
	g.LogRecordMode = mode
}

func (g *GinTrace) AddHook(hook Hook) {
	g.hook = hook
}

func (g *GinTrace) TraceHandler(c *gin.Context) {
	writer := responseWriter{
		c.Writer,
		bytes.NewBuffer([]byte{}),
	}
	c.Writer = writer
	traceId := c.GetHeader("TRACE-ID")
	if len(traceId) == 0 {
		traceId = uuid.New().String()
	}

	t := time.Now()
	trace := &Trace{
		TraceId: traceId,
		Start:   t.Unix(),
	}
	g.trace = trace
	g.hook.BeforeProcess(trace)
	c.Set(trackCtxName, trace)

	c.Next()

	trace.Response = &LinkTraceResponse{
		Header:   writer.Header(),
		Body:     writer.b.String(),
		HttpCode: writer.Status(),
	}
	trace.Request = &LinkTraceRequest{
		Method: c.Request.Method,
		Url:    c.Request.URL.String(),
		Header: c.Request.Header,
		Body:   c.Request.Body,
	}
	if writer.Status() == http.StatusOK {
		trace.Success = true
	}

	trace.End = time.Now().Unix()
	trace.Cost = time.Since(t).String()
	str, err := json.Marshal(&trace)
	if err != nil {
		Error("link trace json marshal error:" + err.Error())
		return
	}

	g.hook.AfterProcess(trace)

	if g.LogFileName == "" {
		return
	}
	switch g.LogRecordMode {
	case "LOCAL":
		UseOtherLog(g.LogFileName, UseLocal()).TranceInfo(string(str))
	case "REMOTE":
		UseOtherLog(g.LogFileName, UseRemote()).TranceInfo(string(str))
	case "CONSOLE":
		UseOtherLog(g.LogFileName, UseConsole()).TranceInfo(string(str))
	}
}
