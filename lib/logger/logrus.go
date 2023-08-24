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

	// 创建一个新的map，包含固定字段和一个名为"details"的新字段，该字段包含所有其他字段
	newData := make(map[string]interface{})

	// 定义固定字段的集合
	fixedFields := map[string]bool{
		"level":    true,
		"trace_id": true,
		"opt":      true,
		"msg":      true,
		"time":     true,
		"file":     true,
		"func":     true,
		"logger":   true,
	}

	// 将其他所有字段添加到"details"字段
	details := make(map[string]interface{})
	for key, value := range data {
		if fixedFields[key] {
			newData[key] = value
		} else {
			details[key] = value
		}
	}

	if len(details) > 0 {
		newData["details"] = details

	}

	// 将更改后的map重新编码为JSON
	jsonBytes, err := json.Marshal(newData)
	if err != nil {
		return nil, err
	}

	// 添加换行符
	return append(jsonBytes, '\n'), nil
}

// Fire 从entry中获取上下文，设置traceID
func (hook LogTrace) Fire(entry *logrus.Entry) error {
	ctx := entry.Context
	if ctx != nil {
		// 从上下文获取一些固定的需要记录的字段
		traceID := ctx.Value("trace_id")
		if traceID != nil {
			entry.Data["trace_id"] = traceID
		}

		opt := ctx.Value("opt")
		if opt != nil {
			entry.Data["opt"] = opt
		} else {
			entry.Data["opt"] = "N/A"
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
