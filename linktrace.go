package fit

import (
	"bytes"
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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
}

// LinkTraceResponse response
type LinkTraceResponse struct {
	Header   interface{} `json:"header"`
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

// LinkTraceExternal record external operations
type LinkTraceExternal struct {
	Url     string      `json:"url"`
	Type    string      `json:"type"`
	Request interface{} `json:"request"`
	Start   int64       `json:"start"`
	End     int64       `json:"end"`
	Error   error       `json:"error"`
	Cost    string      `json:"cost"`
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
	ServiceName        string               `json:"service_name"`
	ServiceType        string               `json:"service_type"`
	TraceId            string               `json:"trace_id"`
	SourceIp           string               `json:"source_ip"`
	Request            *LinkTraceRequest    `json:"request"`
	Response           *LinkTraceResponse   `json:"response"`
	External           []*LinkTraceExternal `json:"external"`
	ThirdPartyRequests []*LinkTraceDialog   `json:"third_party_requests"`
	Error              error                `json:"error"`
	SQLs               []*LinkTraceSQL      `json:"sqls"`
	Redis              []*LinkTraceRedis    `json:"redis"`
	Success            bool                 `json:"success"`
	Start              int64                `json:"start"`
	End                int64                `json:"end"`
	Cost               string               `json:"cost"`
	Extend             map[string]any       `json:"extend"`
	LogRows            []any                `json:"log_rows"`
}

func (t *Trace) AppendSQL(sqlInfo *LinkTraceSQL) {
	t.SQLs = append(t.SQLs, sqlInfo)
}

func (t *Trace) AppendRedis(row *LinkTraceRedis) {
	t.Redis = append(t.Redis, row)
}

func (t *Trace) AppendThirdPartyReq(row *LinkTraceDialog) {
	t.ThirdPartyRequests = append(t.ThirdPartyRequests, row)
}

func (t *Trace) Set(key string, value any) {
	if t.Extend == nil {
		t.Extend = make(map[string]any)
	}
	t.mux.Lock()
	defer t.mux.Unlock()
	t.Extend[key] = value
}

func (t *Trace) AppendLogRow(row any) {
	if t.LogRows == nil {
		t.LogRows = make([]any, 0)
	}
	t.LogRows = append(t.LogRows, row)
}

func (t *Trace) NewLogInfo(row H) H {
	h := H{"level": "info", "time": time.Now().Format("2006-01-02 15:04:05")}
	for k, v := range row {
		h[k] = v
	}
	return h
}

func (t *Trace) NewLogError(row H) H {
	h := H{"level": "error", "time": time.Now().Format("2006-01-02 15:04:05")}
	for k, v := range row {
		h[k] = v
	}
	return h
}

func (t *Trace) NewLogWarning(row H) H {
	h := H{"level": "warning", "time": time.Now().Format("2006-01-02 15:04:05")}
	for k, v := range row {
		h[k] = v
	}
	return h
}

const trackCtxName string = "FIT_TRACE_CTX"

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

type GrpcHookHandler func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error)

type LinkTrace struct {
	LogFileName   string
	LogRecordMode []string
	serviceName   string
	serviceType   string
	Func          func(*Trace)
	trace         *Trace
	hook          Hook
	grpcHook      GrpcHookHandler
	env           EnvType
	DevOutputNO   bool
}

// NewLinkTrace create a new tracker.
func NewLinkTrace(fileName ...string) *LinkTrace {
	logFileName := "trace"
	if len(fileName) > 0 {
		logFileName = fileName[0]
	}
	return &LinkTrace{
		env:         EnvDevelopment,
		LogFileName: logFileName,
	}
}

func (g *LinkTrace) SetRecordMode(modes ...string) {
	g.LogRecordMode = modes
}

func (g *LinkTrace) SetDevOutputNO() {
	g.DevOutputNO = true
}

func (g *LinkTrace) SetServiceName(name string) {
	g.serviceName = name
}

func (g *LinkTrace) SetServiceType(t string) {
	g.serviceType = t
}

func (g *LinkTrace) AddHook(hook Hook) {
	g.hook = hook
}

func (g *LinkTrace) GrpcHook(fn GrpcHookHandler) {
	g.grpcHook = fn
}

func (g *LinkTrace) GinTraceHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if g.DevOutputNO && g.env == EnvDevelopment {
			c.Next()
			return
		}

		writer := responseWriter{
			c.Writer,
			bytes.NewBuffer([]byte{}),
		}
		c.Writer = writer
		traceId := c.GetHeader("FIT-TRACE-ID")
		if len(traceId) == 0 {
			traceId = uuid.New().String()
		}

		t := time.Now()
		trace := &Trace{
			TraceId:     traceId,
			Start:       t.Unix(),
			ServiceName: g.serviceName,
			ServiceType: g.serviceType,
			SourceIp:    c.ClientIP(),
		}
		g.trace = trace
		if g.hook != nil {
			g.hook.BeforeProcess(trace)
		}
		c.Set(trackCtxName, trace)

		c.Next()

		trace.Request = &LinkTraceRequest{
			Method: c.Request.Method,
			Url:    c.Request.URL.String(),
			Header: c.Request.Header,
		}
		trace.Response = &LinkTraceResponse{
			Header:   writer.Header(),
			HttpCode: writer.Status(),
		}
		if writer.Status() == http.StatusOK {
			trace.Success = true
		}

		trace.End = time.Now().Unix()
		trace.Cost = time.Since(t).String()

		if g.hook != nil {
			g.hook.AfterProcess(trace)
		}

		if g.LogFileName == "" {
			return
		}
		for _, kv := range g.LogRecordMode {
			switch kv {
			case "LOCAL":
				OtherLog(g.LogFileName, UseLocal()).TranceInfo(trace)
			case "REMOTE":
				OtherLog(g.LogFileName, UseRemote()).TranceInfo(trace)
			case "CONSOLE":
				OtherLog(g.LogFileName, UseConsole()).TranceInfo(trace)
			}
		}
	}
}

func (g *LinkTrace) GrpcServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if g.DevOutputNO && g.env == EnvDevelopment {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, errors.New("metadata.FromIncomingContext get fail")
		}

		var traceId string
		fitTraceId, ok := md["fit-trace-id"]
		if !ok || fitTraceId[0] == "" {
			traceId = uuid.New().String()
		} else {
			traceId = fitTraceId[0]
		}

		t := time.Now()
		trace := &Trace{
			TraceId:     traceId,
			Start:       t.Unix(),
			ServiceName: g.serviceName,
			ServiceType: g.serviceType,
		}
		g.trace = trace
		if g.hook != nil {
			g.hook.BeforeProcess(trace)
		}
		ctx = context.WithValue(ctx, trackCtxName, trace)

		var res interface{}
		var err error
		if g.grpcHook != nil {
			res, err = g.grpcHook(ctx, req, info, handler)
		} else {
			res, err = handler(ctx, req)
		}

		trace.Request = &LinkTraceRequest{
			Method: info.FullMethod,
			Header: md,
		}

		trace.Error = err
		trace.End = time.Now().Unix()
		trace.Cost = time.Since(t).String()

		if g.hook != nil {
			g.hook.AfterProcess(trace)
		}

		if g.LogFileName == "" {
			return res, err
		}
		for _, kv := range g.LogRecordMode {
			switch kv {
			case "LOCAL":
				OtherLog(g.LogFileName, UseLocal()).TranceInfo(trace)
			case "REMOTE":
				OtherLog(g.LogFileName, UseRemote()).TranceInfo(trace)
			case "CONSOLE":
				OtherLog(g.LogFileName, UseConsole()).TranceInfo(trace)
			}
		}
		return res, err
	}
}

func WithGrpcCtx() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		trace, ok := ctx.Value(trackCtxName).(*Trace)
		var startT time.Time
		if ok {
			ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("FIT-TRACE-ID", trace.TraceId))
			startT = time.Now()
		}
		err := invoker(ctx, method, req, reply, cc, opts...)
		if ok {
			trace.External = append(trace.External, &LinkTraceExternal{
				Url:     method,
				Type:    "gRPC Client",
				Request: req,
				Start:   startT.Unix(),
				End:     time.Now().Unix(),
				Error:   err,
				Cost:    time.Since(startT).String(),
			})
		}
		return err
	}
}
