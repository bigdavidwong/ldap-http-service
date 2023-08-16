package logger

import (
	"encoding/json"
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

type UpperCaseJSONFormatter struct {
	logrus.JSONFormatter
}

func (f *UpperCaseJSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// 使用默认的JSONFormatter
	dataBytes, err := f.JSONFormatter.Format(entry)
	if err != nil {
		return nil, err
	}

	// 将JSON解码为map
	var data map[string]interface{}
	err = json.Unmarshal(dataBytes, &data)
	if err != nil {
		return nil, err
	}

	// 修改级别为大写
	if level, ok := data["level"].(string); ok {
		data["level"] = strings.ToUpper(level)
	}

	// 将更改后的map重新编码为JSON
	return json.Marshal(data)
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
	baseLogger.SetFormatter(&UpperCaseJSONFormatter{})
	baseLogger.SetReportCaller(true)

	GinLogger = baseLogger.WithField("logger", "gin")
	LdapLogger = baseLogger.WithField("logger", "ldap")
}
