package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
)

type Logger struct {
	logger    *log.Logger
	logLevel  LogLevel
	logFormat string
	name      string
}

var (
	loggersMutex sync.Mutex
	loggers      = make(map[string]*Logger)
	logFormat    = "{timestamp} | {level} | {traceId} | {threadId} | {file}:{line} | {func} | {message}"
	logLevel     = DEBUG
)

func NewLogger(name string) (*Logger, error) {
	loggersMutex.Lock()
	defer loggersMutex.Unlock()

	if logger, ok := loggers[name]; ok {
		return logger, nil
	}

	logger := &Logger{
		logger:    log.New(os.Stdout, "", 0), // 直接输出到os.Stdout
		logLevel:  logLevel,
		logFormat: logFormat,
		name:      name,
	}

	loggers[name] = logger
	return logger, nil
}

func (l *Logger) formatEntry(ctx context.Context, level LogLevel, message string) string {
	if level < l.logLevel {
		return ""
	}
	traceId, _ := ctx.Value("traceId").(string)

	pc, file, line, ok := runtime.Caller(3)
	if !ok {
		file = "???"
		line = 0
	}

	funcName := "???"
	if fn := runtime.FuncForPC(pc); fn != nil {
		funcName = fn.Name()
	}

	threadId := runtime.NumGoroutine()
	timestamp := time.Now().Format("2006-01-02 15:04:05.999999")

	replacer := strings.NewReplacer(
		"{timestamp}", timestamp,
		"{level}", logLevelToString(level),
		"{traceId}", traceId,
		"{threadId}", fmt.Sprintf("%d", threadId),
		"{file}", file,
		"{line}", fmt.Sprintf("%d", line),
		"{func}", funcName,
		"{message}", message,
	)

	return replacer.Replace(l.logFormat)
}

func logLevelToString(level LogLevel) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func (l *Logger) Log(ctx context.Context, level LogLevel, message string) {
	formattedMessage := l.formatEntry(ctx, level, message)
	if formattedMessage != "" {
		l.logger.Println(formattedMessage)
	}
}

func (l *Logger) Debug(ctx context.Context, message string) {
	l.Log(ctx, DEBUG, message)
}

func (l *Logger) Info(ctx context.Context, message string) {
	l.Log(ctx, INFO, message)
}

func (l *Logger) Warning(ctx context.Context, message string) {
	l.Log(ctx, WARNING, message)
}

func (l *Logger) Error(ctx context.Context, message string) {
	l.Log(ctx, ERROR, message)
}

func (l *Logger) Fatal(ctx context.Context, message string) {
	l.Log(ctx, ERROR, message)
	os.Exit(0)
}
