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

var customizeLog chan map[string]interface{}

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

func getBody(v ...interface{}) map[string]interface{} {
	var body string
	if len(v) == 1 {
		body = v[0].(string)
		return map[string]interface{}{"message": body}
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

	return SliceConvertMap(v)
}

func writeLocalLog(level int, body map[string]interface{}, rc ...reportCaller) {
	el, ok := logs[defLog]
	if !ok {
		return
	}

	writeFile(el, level, body, rc)
}

func writeLocalLogToJson(level int, body map[string]interface{}, rc ...reportCaller) {
	el, ok := logs[defLog]
	if !ok {
		return
	}

	writeJsonToFile(el, level, body, rc)
}

func writeLocalLogInstance(instance string, level int, body map[string]interface{}, rc ...reportCaller) {
	el, ok := logs[instance]
	if !ok {
		return
	}

	writeFile(el, level, body, rc)
}

func writeFile(el *logrus.Logger, level int, body map[string]interface{}, rc []reportCaller) {
	if len(rc) > 0 && len(rc[0].file) > 0 {
		_, file := filepath.Split(rc[0].file)
		caller := StringSpliceTag(":", file, strconv.Itoa(rc[0].line))
		body["caller"] = caller
		entry := el.WithFields(body)
		switch level {
		case ErrorLevel:
			entry.Error()
		case WarningLevel:
			entry.Warning()
		case FatalLevel:
			entry.Fatal()
		case InfoLevel:
			entry.Info()
		case TranceInfoLevel:
			delete(body, caller)
			rest, err := json.Marshal(&body)
			if err != nil {
				return
			}
			el.WithFields(logrus.Fields{"caller": caller, "trace": string(rest)}).Info()
		}
		return
	}

	entry := el.WithFields(body)
	switch level {
	case ErrorLevel:
		entry.Error()
	case WarningLevel:
		entry.Warning()
	case FatalLevel:
		entry.Fatal()
	case InfoLevel:
		entry.Info()
	}
}

func writeJsonToFile(el *logrus.Logger, level int, body map[string]interface{}, rc []reportCaller) {
	if len(rc) > 0 && len(rc[0].file) > 0 {
		_, file := filepath.Split(rc[0].file)
		caller := StringSpliceTag(":", file, strconv.Itoa(rc[0].line))
		body["caller"] = caller
		jsonText, err := json.Marshal(&body)
		if err != nil {
			return
		}
		entry := el.WithFields(logrus.Fields{"json": string(jsonText)})
		switch level {
		case ErrorLevel:
			entry.Error()
		case WarningLevel:
			entry.Warning()
		case FatalLevel:
			entry.Fatal()
		case InfoLevel:
			entry.Info()
		case TranceInfoLevel:
			delete(body, caller)
			rest, err := json.Marshal(&body)
			if err != nil {
				return
			}
			el.WithFields(logrus.Fields{"caller": caller, "trace": string(rest)}).Info()
		}
		return
	}

	jsonText, err := json.Marshal(&body)
	if err != nil {
		return
	}
	entry := el.WithFields(logrus.Fields{"json": string(jsonText)})
	switch level {
	case ErrorLevel:
		entry.Error()
	case WarningLevel:
		entry.Warning()
	case FatalLevel:
		entry.Fatal()
	case InfoLevel:
		entry.Info()
	}
}

func output(level int, v ...interface{}) {
	if outConsole {
		fmt.Println(v...)
	}

	defer func() {
		if err := recover(); err != nil {
			writeLocalLog(ErrorLevel, H{"msg": "exception capture,an error occurred in the output function", "err": err})
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
			if len(caller.file) > 0 {
				writeLocalLog(ErrorLevel, H{"msg": "NewRabbitMQ() error", "err": err.Error()}, caller)
			} else {
				writeLocalLog(ErrorLevel, H{"msg": "NewRabbitMQ() error", "err": err.Error()})
			}
			return
		}
		defer mq.Close()

		rest, err := json.Marshal(&H{"logger": &body})
		if err != nil {
			if len(caller.file) > 0 {
				writeLocalLog(ErrorLevel, H{"msg": "NewRabbitMQ() error", "err": err.Error()}, caller)
			} else {
				writeLocalLog(ErrorLevel, H{"msg": "NewRabbitMQ() error", "err": err.Error()})
			}
		}

		message := string(rest)
		if remoteTemplateHandler != nil {
			remoteTemplateHandler.Before(message)
		}
		err = mq.DefExchangeDeclare(remoteRabbitMQLog.Exchange, KIND_DIRECT, remoteRabbitMQLog.Durable, true).Publish(message, remoteRabbitMQLog.Key)
		if err != nil {
			if remoteTemplateHandler != nil {
				remoteTemplateHandler.Error(err)
			}
			if len(caller.file) > 0 {
				writeLocalLog(ErrorLevel, H{"msg": "Publish() error", "err": err.Error()}, caller)
			} else {
				writeLocalLog(ErrorLevel, H{"msg": "Publish() error", "err": err.Error()})
			}
		}
	}
}

func outputJSON(level int, s map[string]interface{}) {
	if outConsole {
		fmt.Println(s)
	}

	defer func() {
		if err := recover(); err != nil {
			writeLocalLog(ErrorLevel, H{"message": "exception capture,an error occurred in the output function", "err": err})
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
		writeLocalLogToJson(level, s, caller)
	} else {
		writeLocalLogToJson(level, s)
	}

	//remote log
	if remoteRabbitMQLog != nil {
		mq, err := NewRabbitMQ()
		if err != nil {
			writeLocalLog(ErrorLevel, H{"title": "NewRabbitMQ() error", "err": err.Error()})
			return
		}
		defer mq.Close()

		rest, err := json.Marshal(&H{"logger": &s})
		if err != nil {
			if len(caller.file) > 0 {
				writeLocalLog(ErrorLevel, H{"msg": "NewRabbitMQ() error", "err": err.Error()}, caller)
			} else {
				writeLocalLog(ErrorLevel, H{"msg": "NewRabbitMQ() error", "err": err.Error()})
			}
		}
		message := string(rest)
		if remoteTemplateHandler != nil {
			remoteTemplateHandler.Before(message)
		}
		err = mq.DefExchangeDeclare(remoteRabbitMQLog.Exchange, KIND_DIRECT, remoteRabbitMQLog.Durable, true).
			Publish(message, remoteRabbitMQLog.Key)
		if err != nil {
			if remoteTemplateHandler != nil {
				remoteTemplateHandler.Error(err)
			}
			writeLocalLog(ErrorLevel, H{"title": "Publish() error", "err": err.Error()})
			return
		}
	}
}

func Info(v ...interface{}) {
	output(InfoLevel, v...)
}

func InfoJSON(h map[string]interface{}) {
	outputJSON(InfoLevel, h)
}

func Error(v ...interface{}) {
	output(ErrorLevel, v...)
}

func ErrorJSON(h map[string]interface{}) {
	outputJSON(ErrorLevel, h)
}

func Warning(v ...interface{}) {
	output(WarningLevel, v...)
}

func WarningJSON(h map[string]interface{}) {
	outputJSON(ErrorLevel, h)
}

func Fatal(v ...interface{}) {
	output(FatalLevel, v...)
}

func FatalJSON(h map[string]interface{}) {
	output(FatalLevel, h)
}

func SetOutputToConsole(v bool) {
	outConsole = v
}

func SetRemoteRabbitMQLog(config *RemoteRabbitMQLog) {
	remoteRabbitMQLog = config
}

func CustomizeLog() <-chan map[string]interface{} {
	if customizeLog == nil {
		customizeLog = make(chan map[string]interface{})
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

func (u *useOtherConfig) TranceInfo(v interface{}) {
	u.output(TranceInfoLevel, v)
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
			u.writeLocalLog(ErrorLevel, H{"message": "exception capture,an error occurred in the output function", "err": err})
		}
	}()

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

	var body map[string]interface{}
	if level == TranceInfoLevel {
		if len(v) == 1 {
			u.writeLocalLogTrance(v[0])
		}
	} else {
		body = getBody(v...)
		if len(caller.file) > 0 {
			u.writeLocalLog(level, body, caller)
		} else {
			u.writeLocalLog(level, body)
		}
	}

	//remote log
	if u.remote {
		if body == nil {
			return
		}

		mq, err := NewRabbitMQ()
		if err != nil {
			by := H{"message": "NewRabbitMQ() error", "err": err.Error()}
			if len(caller.file) > 0 {
				u.writeLocalLog(ErrorLevel, by, caller)
			} else {
				u.writeLocalLog(ErrorLevel, by)
			}
			return
		}
		defer mq.Close()

		var message string
		if level == TranceInfoLevel {
			if len(v) == 1 {
				text, err := json.Marshal(&H{"trace": v[0]})
				if err != nil {
					by := H{"message": "NewRabbitMQ() json Marshal err", "err": err.Error()}
					if len(caller.file) > 0 {
						u.writeLocalLog(ErrorLevel, by, caller)
					} else {
						u.writeLocalLog(ErrorLevel, by)
					}
					return
				}
				message = string(text)
			}
		} else {
			str, err := json.Marshal(&H{"logger": &body})
			if err != nil {
				by := H{"message": "NewRabbitMQ() json Marshal err", "err": err.Error()}
				if len(caller.file) > 0 {
					u.writeLocalLog(ErrorLevel, by, caller)
				} else {
					u.writeLocalLog(ErrorLevel, by)
				}
				return
			}
			message = string(str)
		}

		if remoteTemplateHandler != nil {
			remoteTemplateHandler.Before(message)
		}
		err = mq.DefExchangeDeclare(remoteRabbitMQLog.Exchange, KIND_DIRECT, remoteRabbitMQLog.Durable, true).
			Publish(message, remoteRabbitMQLog.Key)
		if err != nil {
			if remoteTemplateHandler != nil {
				remoteTemplateHandler.Error(err)
			}
			by := H{"message": "Publish() error", "err": err.Error()}
			if len(caller.file) > 0 {
				u.writeLocalLog(ErrorLevel, by, caller)
			} else {
				u.writeLocalLog(ErrorLevel, by)
			}

		}
	}
}

func (u *useOtherConfig) writeLocalLogTrance(v interface{}) {
	if u.log == nil {
		return
	}
	u.log.WithFields(logrus.Fields{"trace": v}).Info()
}

func (u *useOtherConfig) writeLocalLog(level int, body map[string]interface{}, rc ...reportCaller) {
	if u.log == nil {
		return
	}
	if len(rc) > 0 && len(rc[0].file) > 0 {
		_, file := filepath.Split(rc[0].file)
		caller := StringSpliceTag(":", file, strconv.Itoa(rc[0].line))
		body["caller"] = caller
		entry := u.log.WithFields(body)
		switch level {
		case ErrorLevel:
			entry.Error()
		case WarningLevel:
			entry.Warning()
		case FatalLevel:
			entry.Fatal()
		case InfoLevel:
			entry.Info()
		}
		return
	}

	entry := u.log.WithFields(body)
	switch level {
	case ErrorLevel:
		entry.Error()
	case WarningLevel:
		entry.Warning()
	case FatalLevel:
		entry.Fatal()
	case InfoLevel:
		entry.Info()
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
	if remoteRabbitMQLog == nil || len(v) == 0 {
		return
	}

	var caller reportCaller
	if isReportCaller {
		if _, file, line, ok := runtime.Caller(2); ok {
			caller.file = file
			caller.line = line
		}
	}

	body := getBody(v...)

	mq, err := NewRabbitMQ()
	if err != nil {
		if len(caller.file) > 0 {
			writeLocalLog(t, H{"message": "Publish() error", "err": err.Error()}, caller)
		} else {
			writeLocalLog(t, H{"message": "Publish() error", "err": err.Error()})
		}
		return
	}
	defer mq.Close()

	str, err := json.Marshal(&H{"logger": &body})
	if err != nil {
		if len(caller.file) > 0 {
			writeLocalLog(t, H{"message": "NewRabbitMQ() json Marshal err", "err": err.Error()}, caller)
		} else {
			writeLocalLog(t, H{"message": "NewRabbitMQ() json Marshal err", "err": err.Error()})
		}
		return
	}
	message := string(str)
	if remoteTemplateHandler != nil {
		remoteTemplateHandler.Before(message)
	}
	err = mq.DefExchangeDeclare(remoteRabbitMQLog.Exchange, KIND_DIRECT, remoteRabbitMQLog.Durable, true).
		Publish(message, remoteRabbitMQLog.Key)
	if err != nil {
		if remoteTemplateHandler != nil {
			remoteTemplateHandler.Error(err)
		}
		if len(caller.file) > 0 {
			writeLocalLog(t, H{"message": "Publish() error", "err": err.Error()}, caller)
		} else {
			writeLocalLog(t, H{"message": "Publish() error", "err": err.Error()})
		}
	}
}
