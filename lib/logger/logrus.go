package logger

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"
)

// LogTrace 定义结构体用于实现Hook方法执行traceID打印
type LogTrace struct {
}

func NewLogTrace() LogTrace {
	return LogTrace{}
}

func (hook LogTrace) Levels() []logrus.Level {
	return logrus.AllLevels
}

type MyFormatter struct{}

func (m *MyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	level := strings.ToUpper(entry.Level.String()) // 将日志级别转换为大写

	traceIDStr := ""
	if traceID, ok := entry.Data["trace_id"]; ok {
		traceIDStr = fmt.Sprintf("%v", traceID) // 如果存在，则添加trace_id字段
	} else {
		traceIDStr = "default_" + strings.Repeat("*", 32)
	}

	loggerStr := ""
	if logger, ok := entry.Data["logger"]; ok {
		loggerStr = fmt.Sprintf("%v", logger) // 如果存在，则添加trace_id字段
	} else {
		loggerStr = "default"
	}

	// 固定字段
	callerStr := ""
	if entry.Caller != nil {
		line := entry.Caller.Line
		function := entry.Caller.Function
		callerStr = fmt.Sprintf("caller=%s:%d", function, line)
	}

	newLog := fmt.Sprintf("[%s] [trace_id=%s] [logger=%s] %s --> ", level, traceIDStr, loggerStr, entry.Message)
	// 弹性地添加其他标签
	for key, value := range entry.Data {
		// 跳过已经添加的字段
		if key == "trace_id" || key == "logger" {
			continue
		}
		newLog += fmt.Sprintf("%s=%v ", key, value)
	}

	newLog += fmt.Sprintf("%s\n", callerStr)

	b.WriteString(newLog)
	return b.Bytes(), nil
}

// Fire 从entry中获取上下文，设置traceID
func (hook LogTrace) Fire(entry *logrus.Entry) error {
	ctx := entry.Context
	if ctx != nil {
		traceID := ctx.Value("trace_id")
		if traceID != nil {
			entry.Data["trace_id"] = traceID
		}
	}
	return nil
}

var (
	GinLogger  *logrus.Entry
	LdapLogger *logrus.Entry
)

func init() {
	// 创建一个Hook跟踪日志
	hook := NewLogTrace()

	baseLogger := logrus.New()
	baseLogger.AddHook(hook)
	baseLogger.SetLevel(logrus.DebugLevel)
	baseLogger.SetFormatter(&MyFormatter{})
	baseLogger.SetReportCaller(true)

	GinLogger = baseLogger.WithField("logger", "gin")
	LdapLogger = baseLogger.WithField("logger", "ldap")
}
