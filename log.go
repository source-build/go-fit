package fit

import (
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/natefinch/lumberjack"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
	"unicode/utf8"
)

var defLog string

var isReportCaller bool

var logs map[string]*logrus.Logger

var outConsole bool

var remoteRabbitMQLog *RemoteRabbitMQLog

var customizeLog chan map[string]interface{}

var stackLength = 300

type LogLevel uint8

var globalLogLevel = InfoLevel

const (
	PanicLevel LogLevel = iota
	FatalLevel
	ErrorLevel
	WarnLevel
	InfoLevel
	DebugLevel
	TranceInfoLevel
)

const (
	JSONFormatter = iota
	TextFormatter
)

var remoteTemplateHandler RemoteLogTemplater

type RemoteRabbitMQLog struct {
	RabbitMQUrl string
	Exchange    string
	Key         string
	Kind        string
	Durable     bool
	AutoDel     bool
	Simple      bool
	MaxConnAt   int64
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
	NoColor      bool
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
	Debug(v ...interface{})
	Info(v ...interface{})
	Error(v ...interface{})
	Warning(v ...interface{})
	Fatal(v ...interface{})
}

type reportCaller struct {
	join string
}

type RemoteLogTemplater interface {
	Before(body map[string]interface{}) map[string]interface{}
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

func (l Local) Debug(v ...interface{}) {
	var caller reportCaller
	if _, file, line, ok := runtime.Caller(1); ok {
		_, fileName := filepath.Split(file)
		caller.join = StringSpliceTag(":", fileName, strconv.Itoa(line))
	}
	writeLocalLogInstance(l.instanceName, DebugLevel, getBody(v...), caller)
}

func (l Local) Info(v ...interface{}) {
	var caller reportCaller
	if _, file, line, ok := runtime.Caller(1); ok {
		_, fileName := filepath.Split(file)
		caller.join = StringSpliceTag(":", fileName, strconv.Itoa(line))
	}
	writeLocalLogInstance(l.instanceName, InfoLevel, getBody(v...), caller)
}

func (l Local) Error(v ...interface{}) {
	var caller reportCaller
	if _, file, line, ok := runtime.Caller(1); ok {
		_, fileName := filepath.Split(file)
		caller.join = StringSpliceTag(":", fileName, strconv.Itoa(line))
	}
	writeLocalLogInstance(l.instanceName, ErrorLevel, getBody(v...), caller)
}

func (l Local) Warning(v ...interface{}) {
	var caller reportCaller
	if _, file, line, ok := runtime.Caller(1); ok {
		_, fileName := filepath.Split(file)
		caller.join = StringSpliceTag(":", fileName, strconv.Itoa(line))
	}
	writeLocalLogInstance(l.instanceName, WarnLevel, getBody(v...), caller)
}

func (l Local) Fatal(v ...interface{}) {
	var caller reportCaller
	if _, file, line, ok := runtime.Caller(1); ok {
		_, fileName := filepath.Split(file)
		caller.join = StringSpliceTag(":", fileName, strconv.Itoa(line))
	}
	writeLocalLogInstance(l.instanceName, FatalLevel, getBody(v...), caller)
}

type LogBodyContent struct {
	err  string
	msg  string
	text string
}

func getBody(v ...interface{}) map[string]interface{} {
	var body string
	if len(v) == 1 {
		eVal, ok := v[0].(error)
		if ok {
			body = eVal.Error()
			if utf8.RuneCountInString(body) > stackLength {
				body = SubStrDecodeRuneInString(body, stackLength)
			}
			return map[string]interface{}{"msg": "An error has occurred", "err": body}
		}

		bVal, ok := v[0].(string)
		if !ok {
			body = ""
			return map[string]interface{}{"msg": body}
		}

		if utf8.RuneCountInString(body) > stackLength {
			body = SubStrDecodeRuneInString(body, stackLength)
		}
		body = bVal
		return map[string]interface{}{"msg": body}
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

func writeLocalLog(level LogLevel, body map[string]interface{}, rc ...reportCaller) {
	el, ok := logs[defLog]
	if !ok {
		return
	}
	writeFile(el, level, body, rc)
}

func writeLocalLogToJson(level LogLevel, body map[string]interface{}, rc ...reportCaller) {
	el, ok := logs[defLog]
	if !ok {
		return
	}

	writeJsonToFile(el, level, body, rc)
}

func writeLocalLogInstance(instance string, level LogLevel, body map[string]interface{}, rc ...reportCaller) {
	el, ok := logs[instance]
	if !ok {
		if len(logs) == 0 {
			return
		}
		for _, v := range logs {
			el = v
			break
		}
	}

	writeFile(el, level, body, rc)
}

func writeFile(el *logrus.Logger, level LogLevel, body map[string]interface{}, rc []reportCaller) {
	var msg string
	if val, ok := body["msg"]; ok {
		if s, o := val.(string); o {
			msg = s
			delete(body, "msg")
		}
	}

	if len(rc) > 0 {
		body["caller"] = rc[0].join
		entry := el.WithFields(body)
		switch level {
		case ErrorLevel:
			entry.Error(msg)
		case WarnLevel:
			entry.Warning(msg)
		case FatalLevel:
			entry.Fatal(msg)
		case InfoLevel:
			entry.Info(msg)
		case DebugLevel:
			entry.Debug(msg)
		case TranceInfoLevel:
			delete(body, rc[0].join)
			rest, err := json.Marshal(&body)
			if err != nil {
				return
			}
			el.WithFields(logrus.Fields{"caller": rc[0].join, "trace": string(rest)}).Info()
		}
		return
	}

	entry := el.WithFields(body)
	switch level {
	case ErrorLevel:
		entry.Error(msg)
	case WarnLevel:
		entry.Warning(msg)
	case FatalLevel:
		entry.Fatal(msg)
	case InfoLevel:
		entry.Info(msg)
	case DebugLevel:
		entry.Debug(msg)
	}
}

func writeJsonToFile(el *logrus.Logger, level LogLevel, body map[string]interface{}, rc []reportCaller) {
	var msg string
	if val, ok := body["msg"]; ok {
		if s, o := val.(string); o {
			msg = s
			delete(body, "msg")
		}
	}

	if len(rc) > 0 {
		jsonText, err := json.Marshal(&body)
		if err != nil {
			return
		}
		entry := el.WithFields(logrus.Fields{"json": string(jsonText), "caller": rc[0].join})
		switch level {
		case ErrorLevel:
			entry.Error(msg)
		case WarnLevel:
			entry.Warning(msg)
		case FatalLevel:
			entry.Fatal(msg)
		case InfoLevel:
			entry.Info(msg)
		case DebugLevel:
			entry.Info(msg)
		case TranceInfoLevel:
			delete(body, rc[0].join)
			rest, err := json.Marshal(&body)
			if err != nil {
				return
			}
			el.WithFields(logrus.Fields{"caller": rc[0].join, "trace": string(rest)}).Info()
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
		entry.Error(msg)
	case WarnLevel:
		entry.Warning(msg)
	case FatalLevel:
		entry.Fatal(msg)
	case InfoLevel:
		entry.Info(msg)
	case DebugLevel:
		entry.Info(msg)
	}
}

func output(level LogLevel, v ...interface{}) {
	var caller reportCaller
	if isReportCaller {
		if _, file, line, ok := runtime.Caller(2); ok {
			_, fileName := filepath.Split(file)
			caller.join = StringSpliceTag(":", fileName, strconv.Itoa(line))
		}
	}

	if outConsole {
		var levelInitial string
		switch level {
		case ErrorLevel:
			levelInitial = "E"
			color.Set(color.FgRed)
		case WarnLevel:
			levelInitial = "W"
			color.Set(color.FgHiYellow)
		case FatalLevel:
			levelInitial = "F"
			color.Set(color.FgHiRed)
		case InfoLevel:
			levelInitial = "I"
			color.Set(color.BgHiMagenta)
		case DebugLevel:
			levelInitial = "D"
			color.Set(color.BgGreen)
		}
		sprintf := fmt.Sprintf("[%s]", levelInitial)
		if isReportCaller {
			sprintf += fmt.Sprintf("[%s]", caller.join)
		}
		sprintf += "\t"
		sprintf += fmt.Sprint(v...)
		fmt.Println(sprintf)
		color.Unset()
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

	// Remote log
	if remoteRabbitMQLog != nil {
		mq, err := getRabbitMQInstance()
		if err != nil {
			if caller.join != "" {
				writeLocalLog(ErrorLevel, H{"msg": "Failed to create rabbitmq!", "err": err.Error()}, caller)
			} else {
				writeLocalLog(ErrorLevel, H{"msg": "Failed to create rabbitmq!", "err": err.Error()})
			}
			return
		}

		if caller.join != "" {
			body["caller"] = caller.join
		}

		if remoteTemplateHandler != nil {
			body = remoteTemplateHandler.Before(body)
		}

		rest, err := json.Marshal(&body)
		if err != nil {
			if caller.join != "" {
				writeLocalLog(ErrorLevel, H{"msg": "JSON serialization failed!", "err": err.Error()}, caller)
			} else {
				writeLocalLog(ErrorLevel, H{"msg": "JSON serialization failed!", "err": err.Error()})
			}
		}

		message := string(rest)

		if remoteRabbitMQLog.Simple {
			err = mq.DefQueueDeclare(remoteRabbitMQLog.Key, remoteRabbitMQLog.Durable, remoteRabbitMQLog.AutoDel).PublishSimple(message)
		} else {
			var key string
			if remoteRabbitMQLog.Kind == KIND_DIRECT {
				key = GetLevelStringByType(level)
			} else {
				key = remoteRabbitMQLog.Key
			}
			err = mq.DefExchangeDeclare(remoteRabbitMQLog.Exchange, remoteRabbitMQLog.Kind, remoteRabbitMQLog.Durable, remoteRabbitMQLog.AutoDel).PublishRouting(message, key)
		}

		if err != nil {
			if remoteTemplateHandler != nil {
				remoteTemplateHandler.Error(err)
			}
			if caller.join != "" {
				writeLocalLog(ErrorLevel, H{"msg": "Remote log sending failed!", "err": err.Error()}, caller)
			} else {
				writeLocalLog(ErrorLevel, H{"msg": "Remote log sending failed!", "err": err.Error()})
			}
		}
	}

	// Local log
	if caller.join != "" {
		writeLocalLog(level, body, caller)
	} else {
		writeLocalLog(level, body)
	}
}

func outputJSON(level LogLevel, s map[string]interface{}) {
	var caller reportCaller
	if isReportCaller {
		if _, file, line, ok := runtime.Caller(2); ok {
			_, fileName := filepath.Split(file)
			caller.join = StringSpliceTag(":", fileName, strconv.Itoa(line))
		}
	}

	if outConsole {
		var levelInitial string
		switch level {
		case ErrorLevel:
			levelInitial = "E"
			color.Set(color.FgRed)
		case WarnLevel:
			levelInitial = "W"
			color.Set(color.FgHiYellow)
		case FatalLevel:
			levelInitial = "F"
			color.Set(color.FgHiRed)
		case InfoLevel:
			levelInitial = "I"
			color.Set(color.BgHiMagenta)
		case DebugLevel:
			levelInitial = "D"
			color.Set(color.BgGreen)
		}
		sprintf := fmt.Sprintf("[%s]", levelInitial)
		if isReportCaller {
			sprintf += fmt.Sprintf("[%s]", caller.join)
		}
		sprintf += "\t"
		sprintf += fmt.Sprint(s)
		fmt.Println(sprintf)
		color.Unset()
	}

	defer func() {
		if err := recover(); err != nil {
			writeLocalLog(ErrorLevel, H{"message": "exception capture,an error occurred in the output function", "err": err})
		}
	}()

	if customizeLog != nil {
		customizeLog <- s
	}

	//remote log
	if remoteRabbitMQLog != nil {
		mq, err := getRabbitMQInstance()
		if err != nil {
			writeLocalLog(ErrorLevel, H{"msg": "Failed to create rabbitmq!", "err": err.Error()})
			return
		}

		if caller.join != "" {
			s["caller"] = caller.join
		}

		if remoteTemplateHandler != nil {
			s = remoteTemplateHandler.Before(s)
		}

		rest, err := json.Marshal(&s)
		if err != nil {
			if caller.join != "" {
				writeLocalLog(ErrorLevel, H{"msg": "JSON serialization failed!", "err": err.Error()}, caller)
			} else {
				writeLocalLog(ErrorLevel, H{"msg": "JSON serialization failed!", "err": err.Error()})
			}
		}

		message := string(rest)

		if remoteRabbitMQLog.Simple {
			err = mq.DefQueueDeclare(remoteRabbitMQLog.Key, remoteRabbitMQLog.Durable, remoteRabbitMQLog.AutoDel).PublishSimple(message)
		} else {
			var key string
			if remoteRabbitMQLog.Kind == KIND_DIRECT {
				key = GetLevelStringByType(level)
			} else {
				key = remoteRabbitMQLog.Key
			}
			err = mq.DefExchangeDeclare(remoteRabbitMQLog.Exchange, remoteRabbitMQLog.Kind, remoteRabbitMQLog.Durable, remoteRabbitMQLog.AutoDel).PublishRouting(message, key)
		}

		if err != nil {
			if remoteTemplateHandler != nil {
				remoteTemplateHandler.Error(err)
			}
			writeLocalLog(ErrorLevel, H{"msg": "Remote log sending failed!", "err": err.Error()})
			return
		}
	}

	if caller.join != "" {
		writeLocalLogToJson(level, s, caller)
	} else {
		writeLocalLogToJson(level, s)
	}
}

func Info(v ...interface{}) {
	output(InfoLevel, v...)
}

func InfoJSON(h map[string]interface{}) {
	outputJSON(InfoLevel, h)
}

func Debug(v ...interface{}) {
	output(DebugLevel, v...)
}

func Error(v ...interface{}) {
	output(ErrorLevel, v...)
}

func DebugJSON(h map[string]interface{}) {
	outputJSON(DebugLevel, h)
}

func ErrorJSON(h map[string]interface{}) {
	outputJSON(ErrorLevel, h)
}

func Warning(v ...interface{}) {
	output(WarnLevel, v...)
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

func SetConsoleLogNoColor() {
	color.NoColor = true
}

func SetOutputToConsole(v bool) {
	if globalLogLevel == DebugLevel {
		outConsole = v
	}
}

type remoteRabbit struct {
	inst      *RabbitMQ
	useTime   time.Time
	createdAt int64
}

var _remoteRabbitInstance *remoteRabbit

func SetRemoteRabbitMQLog(config *RemoteRabbitMQLog) {
	_remoteRabbitInstance = &remoteRabbit{}
	remoteRabbitMQLog = config
}

func getRabbitMQInstance() (*RabbitMQ, error) {
	in := _remoteRabbitInstance
	in.useTime = time.Now()
	if in.inst == nil {
		mq, err := NewRabbitMQ(remoteRabbitMQLog.RabbitMQUrl)
		if err != nil {
			writeLocalLog(ErrorLevel, H{"msg": "Failed to create rabbitmq!", "err": err.Error()})
			return nil, err
		}
		in.inst = mq
		in.createdAt = time.Now().Unix()
		go in.upholdInstance()
	}
	return in.inst, nil
}

func (r *remoteRabbit) upholdInstance() {
	for {
		ct := time.Now().Unix()
		if remoteRabbitMQLog.MaxConnAt > 0 && ct-r.createdAt > remoteRabbitMQLog.MaxConnAt {
			r.inst.Close()
			r.inst = nil
			return
		}
		time.Sleep(time.Second * 2)
		if ct-r.useTime.Unix() > 10 {
			r.inst.Close()
			r.inst = nil
			return
		}
	}
}

func GetLevelStringByType(level LogLevel) string {
	switch level {
	case ErrorLevel:
		return "error"
	case WarnLevel:
		return "warning"
	case InfoLevel:
		return "info"
	case DebugLevel:
		return "debug"
	case PanicLevel:
		return "panic"
	case FatalLevel:
		return "fatal"
	}
	return ""
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

// UseNotReportCaller does not set whether the standard logger will include the calling method as a field.
func UseNotReportCaller() UseOtherFunc {
	return func(c *useOtherConfig) {
		c.caller = true
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

func (u *useOtherConfig) Debug(v ...interface{}) {
	u.output(DebugLevel, v...)
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
	u.output(WarnLevel, v...)
}

func (u *useOtherConfig) Fatal(v ...interface{}) {
	u.output(FatalLevel, v...)
}

func (u *useOtherConfig) output(level LogLevel, v ...interface{}) {
	if u.console {
		fmt.Println(v...)
	}
	defer func() {
		if err := recover(); err != nil {
			u.writeLocalLog(ErrorLevel, H{"msg": "exception capture,an error occurred in the output function", "err": err})
		}
	}()

	var caller reportCaller
	if !u.caller {
		var s int
		if u.skip > 0 {
			s = u.skip
		} else {
			s = 2
		}
		if _, file, line, ok := runtime.Caller(s); ok {
			_, fileName := filepath.Split(file)
			caller.join = StringSpliceTag(":", fileName, strconv.Itoa(line))
		}
	}

	var body map[string]interface{}
	if level == TranceInfoLevel {
		if len(v) == 1 {
			u.writeLocalLogTrance(v[0])
		}
	} else {
		body = getBody(v...)
		if caller.join != "" {
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

		mq, err := getRabbitMQInstance()
		if err != nil {
			by := H{"msg": "Failed to create rabbitmq!", "err": err.Error()}
			if len(caller.join) > 0 {
				u.writeLocalLog(ErrorLevel, by, caller)
			} else {
				u.writeLocalLog(ErrorLevel, by)
			}
			return
		}

		if caller.join != "" {
			body["caller"] = caller.join
		}

		if remoteTemplateHandler != nil {
			body = remoteTemplateHandler.Before(body)
		}

		var message string
		if level == TranceInfoLevel {
			if len(v) == 1 {
				text, err := json.Marshal(&H{"trace": v[0]})
				if err != nil {
					by := H{"msg": "[remote log]:json Marshal err", "err": err.Error()}
					if caller.join != "" {
						u.writeLocalLog(ErrorLevel, by, caller)
					} else {
						u.writeLocalLog(ErrorLevel, by)
					}
					return
				}
				message = string(text)
			}
		} else {
			str, err := json.Marshal(&body)
			if err != nil {
				by := H{"msg": "Failed to create rabbitmq!", "err": err.Error()}
				if caller.join != "" {
					u.writeLocalLog(ErrorLevel, by, caller)
				} else {
					u.writeLocalLog(ErrorLevel, by)
				}
				return
			}
			message = string(str)
		}

		if remoteRabbitMQLog.Simple {
			err = mq.DefQueueDeclare(remoteRabbitMQLog.Key, remoteRabbitMQLog.Durable, remoteRabbitMQLog.AutoDel).PublishSimple(message)
		} else {
			var key string
			if remoteRabbitMQLog.Kind == KIND_DIRECT {
				key = GetLevelStringByType(level)
			} else {
				key = remoteRabbitMQLog.Key
			}
			err = mq.DefExchangeDeclare(remoteRabbitMQLog.Exchange, remoteRabbitMQLog.Kind, remoteRabbitMQLog.Durable, remoteRabbitMQLog.AutoDel).PublishRouting(message, key)
		}

		if err != nil {
			if remoteTemplateHandler != nil {
				remoteTemplateHandler.Error(err)
			}
			by := H{"msg": "Remote log sending failed!", "err": err.Error()}
			if caller.join != "" {
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

func (u *useOtherConfig) writeLocalLog(level LogLevel, body map[string]interface{}, rc ...reportCaller) {
	var msg string
	if val, ok := body["msg"]; ok {
		if s, o := val.(string); o {
			msg = s
			delete(body, "msg")
		}
	}

	if u.log == nil {
		return
	}
	if len(rc) > 0 {
		body["caller"] = rc[0].join
		entry := u.log.WithFields(body)
		switch level {
		case ErrorLevel:
			entry.Error(msg)
		case WarnLevel:
			entry.Warning(msg)
		case FatalLevel:
			entry.Fatal(msg)
		case DebugLevel:
			entry.Fatal(msg)
		case InfoLevel:
			entry.Info(msg)
		}
		return
	}

	entry := u.log.WithFields(body)
	switch level {
	case ErrorLevel:
		entry.Error(msg)
	case WarnLevel:
		entry.Warning(msg)
	case FatalLevel:
		entry.Fatal(msg)
	case DebugLevel:
		entry.Fatal(msg)
	case InfoLevel:
		entry.Info(msg)
	}
}

func AddRemoteLogHook(r RemoteLogTemplater) {
	remoteTemplateHandler = r
}

func SetLogLevel(level LogLevel) {
	globalLogLevel = level
}

func SetLocalLogConfig(entity ...LogEntity) {
	if len(entity) == 1 {
		defLog = entity[0].FileName
	}
	logs = make(map[string]*logrus.Logger)
	for _, k := range entity {
		if _, ok := logs[k.FileName]; ok {
			continue
		}
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
			l.SetFormatter(&logrus.TextFormatter{
				TimestampFormat: "2006-01-02 15:04:05",
			})
		} else {
			l.SetFormatter(&logrus.JSONFormatter{
				TimestampFormat: "2006-01-02 15:04:05",
			})
		}
		l.SetLevel(logrus.Level(globalLogLevel))
		logs[k.FileName] = l
		isReportCaller = !k.ReportCaller
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

func RemoteLog(t LogLevel, v ...interface{}) {
	if remoteRabbitMQLog == nil || len(v) == 0 {
		return
	}

	var caller reportCaller
	if isReportCaller {
		if _, file, line, ok := runtime.Caller(2); ok {
			_, fileName := filepath.Split(file)
			caller.join = StringSpliceTag(":", fileName, strconv.Itoa(line))
		}
	}

	body := getBody(v...)

	mq, err := getRabbitMQInstance()
	if err != nil {
		if caller.join != "" {
			writeLocalLog(t, H{"msg": "connect rabbitMQ error", "err": err.Error()}, caller)
		} else {
			writeLocalLog(t, H{"msg": "connect rabbitMQ error", "err": err.Error()})
		}
		return
	}

	if caller.join != "" {
		body["caller"] = caller.join
	}

	if remoteTemplateHandler != nil {
		body = remoteTemplateHandler.Before(body)
	}

	rest, err := json.Marshal(&body)
	if err != nil {
		if caller.join != "" {
			writeLocalLog(ErrorLevel, H{"msg": "JSON serialization failed!", "err": err.Error()}, caller)
		} else {
			writeLocalLog(ErrorLevel, H{"msg": "JSON serialization failed!", "err": err.Error()})
		}
	}

	message := string(rest)

	if remoteRabbitMQLog.Simple {
		err = mq.DefQueueDeclare(remoteRabbitMQLog.Key, remoteRabbitMQLog.Durable, remoteRabbitMQLog.AutoDel).PublishSimple(message)
	} else {
		var key string
		if remoteRabbitMQLog.Kind == KIND_DIRECT {
			key = GetLevelStringByType(t)
		} else {
			key = remoteRabbitMQLog.Key
		}
		err = mq.DefExchangeDeclare(remoteRabbitMQLog.Exchange, remoteRabbitMQLog.Kind, remoteRabbitMQLog.Durable, remoteRabbitMQLog.AutoDel).PublishRouting(message, key)
	}
	if err != nil {
		if remoteTemplateHandler != nil {
			remoteTemplateHandler.Error(err)
		}
		if caller.join != "" {
			writeLocalLog(t, H{"msg": "Remote log sending failed!", "err": err.Error()}, caller)
		} else {
			writeLocalLog(t, H{"msg": "Remote log sending failed!", "err": err.Error()})
		}
	}
}
