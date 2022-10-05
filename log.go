package fit

import (
	"encoding/json"
	"fmt"
	"github.com/natefinch/lumberjack"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"runtime"
	"strconv"
)

var defLog string

var isReportCaller bool

var logs map[string]*logrus.Logger

var outConsole bool

var remoteRabbitMQLog *RemoteRabbitMQLog

var customizeLog chan string

var stackLength int = 300

const (
	ErrorLevel = iota
	WarningLevel
	FatalLevel
	InfoLevel
	TranceInfoLevel
)

const (
	JSONFormatter = iota
	TextFormatter
)

var remoteTemplateHandler RemoteLogTemplater

type RemoteRabbitMQLog struct {
	Exchange string
	Key      string
	Durable  bool
}

type Fields map[string]any

func (f Fields) ToSlice() []interface{} {
	return MapConvertSlice(f)
}

func (f Fields) ToJSON() string {
	str, err := json.Marshal(f)
	if err != nil {
		return ""
	}

	return string(str)
}

func (f Fields) ToStruct() *interface{} {
	var output interface{}
	err := MapConvertStruct(f, &output)
	if err != nil {
		return nil
	}
	return &output
}

type LogEntity struct {
	// Log save location
	LogPath string
	// Maximum capacity of a single file in MB
	FileMaxSize int
	// Maximum number of expired files retained
	MaxBackups int
	// Maximum time interval for retaining expired files, in days
	MaxAge int
	// Whether the rolling log needs to be compressed, 'gzip' compression
	Compress bool
	// log file name
	FileName     string
	IsDefaultLog bool
	Formatter    int
	ReportCaller bool
}

func GetLogInstances() map[string]*logrus.Logger {
	return logs
}

func GetLogInstance(name string) (LoggerInstance *logrus.Logger, ok bool) {
	logger, ok := logs[name]
	if !ok {
		return nil, false
	}
	return logger, true
}

type Level interface {
	Info(v ...interface{})
	Error(v ...interface{})
	Warning(v ...interface{})
	Fatal(v ...interface{})
}

type reportCaller struct {
	file string
	line int
}

type RemoteLogTemplater interface {
	Before(body string) string
	Error(err error)
}

type Local struct {
	instanceName string
}

func LocalLog(name ...string) Local {
	if len(name) > 0 {
		return Local{instanceName: name[0]}
	}
	return Local{}
}

func (l Local) Info(v ...interface{}) {
	var caller reportCaller
	if _, file, line, ok := runtime.Caller(1); ok {
		caller.file = file
		caller.line = line
	}
	writeLocalLogInstance(l.instanceName, InfoLevel, getBody(v...), caller)
}

func (l Local) Error(v ...interface{}) {
	var caller reportCaller
	if _, file, line, ok := runtime.Caller(1); ok {
		caller.file = file
		caller.line = line
	}
	writeLocalLogInstance(l.instanceName, ErrorLevel, getBody(v...), caller)
}

func (l Local) Warning(v ...interface{}) {
	var caller reportCaller
	if _, file, line, ok := runtime.Caller(1); ok {
		caller.file = file
		caller.line = line
	}
	writeLocalLogInstance(l.instanceName, WarningLevel, getBody(v...), caller)
}

func (l Local) Fatal(v ...interface{}) {
	var caller reportCaller
	if _, file, line, ok := runtime.Caller(1); ok {
		caller.file = file
		caller.line = line
	}
	writeLocalLogInstance(l.instanceName, FatalLevel, getBody(v...), caller)
}

func getBody(v ...interface{}) string {
	var body string
	if len(v) <= 1 {
		body = v[0].(string)
		return body
	}

	//intercept stack information of specified length
	indx := -1
	for i, kv := range v {
		if kv == "err" {
			indx = i + 1
			continue
		}
		if indx == i {
			er, ok := kv.(error)
			if ok {
				v[i] = SubStrDecodeRuneInString(er.Error(), stackLength)
			}
			break
		}
	}

	b, err := json.Marshal(SliceConvertMap(v))
	if err != nil {
		return ""
	}
	body = string(b)

	return body
}

func writeLocalLog(level int, body string, rc ...reportCaller) {
	el, ok := logs[defLog]
	if !ok {
		return
	}

	writeFile(el, level, body, rc)
}

func writeLocalLogInstance(instance string, level int, body string, rc ...reportCaller) {
	el, ok := logs[instance]
	if !ok {
		return
	}

	writeFile(el, level, body, rc)
}

func writeFile(el *logrus.Logger, level int, body string, rc []reportCaller) {
	if len(rc) > 0 && len(rc[0].file) > 0 {
		_, file := filepath.Split(rc[0].file)
		caller := StringSpliceTag(":", file, strconv.Itoa(rc[0].line))
		switch level {
		case ErrorLevel:
			el.WithField("caller", caller).Error(body)
		case WarningLevel:
			el.WithField("caller", caller).Warning(body)
		case FatalLevel:
			el.WithField("caller", caller).Fatal(body)
		case InfoLevel:
			el.WithField("caller", caller).Info(body)
		case TranceInfoLevel:
			el.WithFields(logrus.Fields{"caller": caller, "trace": body}).Info()
		}
		return
	}
	switch level {
	case ErrorLevel:
		el.Error(body)
	case WarningLevel:
		el.Warning(body)
	case FatalLevel:
		el.Fatal(body)
	case InfoLevel:
		el.Info(body)
	}
}

func output(level int, v ...interface{}) {
	if outConsole {
		fmt.Println(v...)
	}

	defer func() {
		if err := recover(); err != nil {
			writeLocalLog(ErrorLevel, H{"title": "exception capture,an error occurred in the output function", "error": err}.ToString())
		}
	}()

	//slice to json
	body := getBody(v...)
	if customizeLog != nil {
		customizeLog <- body
	}

	var caller reportCaller
	if isReportCaller {
		if _, file, line, ok := runtime.Caller(2); ok {
			caller.file = file
			caller.line = line
		}
	}

	if len(caller.file) > 0 {
		writeLocalLog(level, body, caller)
	} else {
		writeLocalLog(level, body)
	}

	//remote log
	if remoteRabbitMQLog != nil {
		mq, err := NewRabbitMQ()
		if err != nil {
			writeLocalLog(ErrorLevel, H{"title": "NewRabbitMQ() error", "error": err.Error()}.ToString())
			return
		}
		defer mq.Close()
		if remoteTemplateHandler != nil {
			body = remoteTemplateHandler.Before(body)
		}
		err = mq.DefExchangeDeclare(remoteRabbitMQLog.Exchange, KIND_DIRECT, remoteRabbitMQLog.Durable, true).
			Publish(body, remoteRabbitMQLog.Key)
		if err != nil {
			if remoteTemplateHandler != nil {
				remoteTemplateHandler.Error(err)
			}
			writeLocalLog(ErrorLevel, H{"title": "Publish() error", "error": err.Error()}.ToString())
			return
		}
	}
}

func outputJSON(level int, s string) {
	if outConsole {
		fmt.Println(s)
	}

	defer func() {
		if err := recover(); err != nil {
			writeLocalLog(ErrorLevel, H{"title": "exception capture,an error occurred in the output function", "error": err}.ToString())
		}
	}()

	if customizeLog != nil {
		customizeLog <- s
	}

	var caller reportCaller
	if isReportCaller {
		if _, file, line, ok := runtime.Caller(2); ok {
			caller.file = file
			caller.line = line
		}
	}

	if len(caller.file) > 0 {
		writeLocalLog(level, s, caller)
	} else {
		writeLocalLog(level, s)
	}

	//remote log
	if remoteRabbitMQLog != nil {
		mq, err := NewRabbitMQ()
		if err != nil {
			writeLocalLog(ErrorLevel, H{"title": "NewRabbitMQ() error", "error": err.Error()}.ToString())
			return
		}
		defer mq.Close()
		if remoteTemplateHandler != nil {
			s = remoteTemplateHandler.Before(s)
		}
		err = mq.DefExchangeDeclare(remoteRabbitMQLog.Exchange, KIND_DIRECT, remoteRabbitMQLog.Durable, true).
			Publish(s, remoteRabbitMQLog.Key)
		if err != nil {
			if remoteTemplateHandler != nil {
				remoteTemplateHandler.Error(err)
			}
			writeLocalLog(ErrorLevel, H{"title": "Publish() error", "error": err.Error()}.ToString())
			return
		}
	}
}

func Info(v ...interface{}) {
	output(InfoLevel, v...)
}

func InfoJSON(h H) {
	outputJSON(InfoLevel, h.ToString())
}

func Error(v ...interface{}) {
	output(ErrorLevel, v...)
}

func ErrorJSON(h H) {
	outputJSON(ErrorLevel, h.ToString())
}

func Warning(v ...interface{}) {
	output(WarningLevel, v...)
}

func WarningJSON(h H) {
	outputJSON(ErrorLevel, h.ToString())
}

func Fatal(v ...interface{}) {
	output(FatalLevel, v...)
}

func FatalJSON(h H) {
	output(FatalLevel, h.ToString())
}

func SetOutputToConsole(v bool) {
	outConsole = v
}

func SetRemoteRabbitMQLog(config *RemoteRabbitMQLog) {
	remoteRabbitMQLog = config
}

func CustomizeLog() <-chan string {
	if customizeLog == nil {
		customizeLog = make(chan string)
	}
	return customizeLog
}

func CloseCustomizeLog() {
	if customizeLog != nil {
		close(customizeLog)
	}
}

type useOtherConfig struct {
	local   bool
	remote  bool
	console bool
	skip    int
	caller  bool
	log     *logrus.Logger
}

type UseOtherFunc func(*useOtherConfig)

func UseLocal() UseOtherFunc {
	return func(c *useOtherConfig) {
		c.local = true
	}
}

func UseRemote() UseOtherFunc {
	return func(c *useOtherConfig) {
		c.remote = true
	}
}

func UseConsole() UseOtherFunc {
	return func(c *useOtherConfig) {
		c.console = true
	}
}

// UseSetSkip number of stack frames to be traced back, default 2.
func UseSetSkip(skip int) UseOtherFunc {
	return func(c *useOtherConfig) {
		c.skip = skip
	}
}

// UseReportCaller sets whether the standard logger will include the calling method as a field.
func UseReportCaller(v bool) UseOtherFunc {
	return func(c *useOtherConfig) {
		c.caller = v
	}
}

func OtherLog(name string, opts ...UseOtherFunc) *useOtherConfig {
	var config useOtherConfig

	for _, opt := range opts {
		opt(&config)
	}

	if config.local {
		if el, ok := logs[name]; ok {
			config.log = el
		}
	}

	return &config
}

func (u *useOtherConfig) Info(v ...interface{}) {
	u.output(InfoLevel, v...)
}

func (u *useOtherConfig) TranceInfo(v ...interface{}) {
	u.output(TranceInfoLevel, v...)
}

func (u *useOtherConfig) Error(v ...interface{}) {
	u.output(ErrorLevel, v...)
}

func (u *useOtherConfig) Warning(v ...interface{}) {
	u.output(WarningLevel, v...)
}

func (u *useOtherConfig) Fatal(v ...interface{}) {
	u.output(FatalLevel, v...)
}

func (u *useOtherConfig) output(level int, v ...interface{}) {
	if u.console {
		fmt.Println(v...)
	}

	defer func() {
		if err := recover(); err != nil {
			u.writeLocalLog(ErrorLevel, H{"title": "exception capture,an error occurred in the output function", "error": err}.ToString())
		}
	}()

	body := getBody(v...)

	var caller reportCaller
	if u.caller {
		var s int
		if u.skip > 0 {
			s = u.skip
		} else {
			s = 2
		}
		if _, file, line, ok := runtime.Caller(s); ok {
			caller.file = file
			caller.line = line
		}
	}

	if len(caller.file) > 0 {
		u.writeLocalLog(level, body, caller)
	} else {
		u.writeLocalLog(level, body)
	}

	//remote log
	if u.remote {
		mq, err := NewRabbitMQ()
		if err != nil {
			u.writeLocalLog(ErrorLevel, H{"title": "NewRabbitMQ() error", "error": err.Error()}.ToString())
			return
		}
		defer mq.Close()
		if remoteTemplateHandler != nil {
			body = remoteTemplateHandler.Before(body)
		}
		err = mq.DefExchangeDeclare(remoteRabbitMQLog.Exchange, KIND_DIRECT, remoteRabbitMQLog.Durable, true).
			Publish(body, remoteRabbitMQLog.Key)
		if err != nil {
			if remoteTemplateHandler != nil {
				remoteTemplateHandler.Error(err)
			}
			u.writeLocalLog(ErrorLevel, H{"title": "Publish() error", "error": err.Error()}.ToString())
			return
		}
	}
}

func (u *useOtherConfig) writeLocalLog(level int, body string, rc ...reportCaller) {
	if u.log == nil {
		return
	}
	if len(rc) > 0 && len(rc[0].file) > 0 {
		_, file := filepath.Split(rc[0].file)
		caller := StringSpliceTag(":", file, strconv.Itoa(rc[0].line))
		switch level {
		case ErrorLevel:
			u.log.WithField("caller", caller).Error(body)
		case WarningLevel:
			u.log.WithField("caller", caller).Warning(body)
		case FatalLevel:
			u.log.WithField("caller", caller).Fatal(body)
		case InfoLevel:
			u.log.WithField("caller", caller).Info(body)
		case TranceInfoLevel:
			u.log.WithFields(logrus.Fields{"caller": caller, "trace": body}).Info()
		}
		return
	}
	switch level {
	case ErrorLevel:
		u.log.Error(body)
	case WarningLevel:
		u.log.Warning(body)
	case FatalLevel:
		u.log.Fatal(body)
	case InfoLevel:
		u.log.Info(body)
	case TranceInfoLevel:
		u.log.WithFields(logrus.Fields{"trace_info": body}).Info()
	}
}

func AddRemoteLogHook(r RemoteLogTemplater) {
	remoteTemplateHandler = r
}

func SetLocalLogConfig(entity ...LogEntity) {
	logs = make(map[string]*logrus.Logger)
	for _, k := range entity {
		defaultConfig(&k)
		if k.IsDefaultLog {
			defLog = k.FileName
		}
		l := logrus.New()
		l.SetOutput(&lumberjack.Logger{
			Filename:   StringSpliceTag("/", k.LogPath, k.FileName+".log"),
			MaxSize:    k.FileMaxSize,
			MaxBackups: k.MaxBackups,
			MaxAge:     k.MaxAge,
			Compress:   k.Compress,
		})
		if k.Formatter == TextFormatter {
			l.SetFormatter(&logrus.TextFormatter{})
		} else {
			l.SetFormatter(&logrus.JSONFormatter{})
		}
		l.SetLevel(logrus.InfoLevel)
		logs[k.FileName] = l
		isReportCaller = k.ReportCaller
	}
}

func SetLogStackLength(len int) {
	if len <= 0 {
		return
	}
	stackLength = len
}

func defaultConfig(entity *LogEntity) *LogEntity {
	if entity.FileName == "" {
		entity.FileName = "general"
	}
	if entity.LogPath == "" {
		entity.LogPath = "logs"
	}
	if entity.MaxAge == 0 {
		entity.MaxAge = 3
	}
	if entity.MaxBackups == 0 {
		entity.MaxBackups = 5
	}
	if entity.FileMaxSize == 0 {
		entity.FileMaxSize = 5
	}
	return entity
}

func RemoteLog(t int, v ...interface{}) {
	if remoteRabbitMQLog != nil {
		body := getBody(v...)
		mq, err := NewRabbitMQ()
		defer mq.Close()
		if err != nil {
			writeLocalLog(t, body)
			return
		}
		if remoteTemplateHandler != nil {
			body = remoteTemplateHandler.Before(body)
		}
		err = mq.DefExchangeDeclare(remoteRabbitMQLog.Exchange, KIND_DIRECT, remoteRabbitMQLog.Durable, true).
			Publish(body, remoteRabbitMQLog.Key)
		if err != nil {
			if remoteTemplateHandler != nil {
				remoteTemplateHandler.Error(err)
			}
			writeLocalLog(t, body)
			return
		}
	}
}
