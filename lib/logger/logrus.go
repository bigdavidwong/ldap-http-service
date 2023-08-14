package logger

import "github.com/sirupsen/logrus"

// LogTrace 定义结构体用于实现Hook方法执行traceID打印
type LogTrace struct {
}

func NewLogTrace() LogTrace {
	return LogTrace{}
}

func (hook LogTrace) Levels() []logrus.Level {
	return logrus.AllLevels
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
	GinLogger  *logrus.Logger
	LdapLogger *logrus.Logger
)

func init() {
	// 创建一个Hook跟踪日志
	hook := NewLogTrace()

	GinLogger = logrus.New()
	GinLogger.SetFormatter(&logrus.JSONFormatter{})
	GinLogger.SetLevel(logrus.InfoLevel)
	GinLogger.AddHook(hook)

	LdapLogger = logrus.New()
	LdapLogger.SetFormatter(&logrus.JSONFormatter{})
	LdapLogger.SetLevel(logrus.WarnLevel)
	LdapLogger.AddHook(hook)
}
