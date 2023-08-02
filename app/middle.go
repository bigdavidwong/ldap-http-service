package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"ldap-http-service/lib/ers"
	"ldap-http-service/lib/logger"
	"ldap-http-service/lib/utils"
	"log"
	"net/http"
	"runtime"
	"runtime/debug"
	"time"
)

type ResponseData struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	TraceId string      `json:"trace_id"`
}

func JsonWithTraceId(c *gin.Context, httpCode, code int, msg string, data interface{}) {
	traceId, _ := c.Value("traceId").(string)
	response := ResponseData{
		Code:    code,
		Message: msg,
		Data:    data,
		TraceId: traceId,
	}

	// 尝试序列化数据，校验是否有错误
	_, err := json.Marshal(data)
	if err != nil {
		log.Panicf("unsupported json body -> %v", err)
	}

	// 打印日志
	log.Printf("Response: %+v\n", response)

	c.JSON(httpCode, response)
}

// processRequest 处理请求中间件，包含错误集中处理、panic恢复以及traceId标记等
func processRequest(logger *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 在gin自带上下文中标记traceId
		traceId := c.GetHeader("trace_id")
		if traceId == "" {
			// 如果 Header 中不存在 "trace_id"，则自动生成
			traceId = utils.GenUuid("req")
		}
		c.Set("traceId", traceId)

		// panic恢复和异常处理
		defer func() {
			if r := recover(); r != nil {
				_, file, line, _ := runtime.Caller(2)
				logger.Warning(c, fmt.Sprintf("Recover from %s:%d -> %v", file, line, r))
				logger.Debug(c, string(debug.Stack()))
				// 使用类型断言检查错误是否为自定义错误
				if customErr, ok := r.(ers.CustomErr); ok {
					JsonWithTraceId(c, customErr.HttpCode(), customErr.Code(), customErr.Error(), nil)
				} else {
					// 其他未知错误
					JsonWithTraceId(c, http.StatusInternalServerError, 1000, "Internal Server Error", map[string]interface{}{})
				}
			}
		}()
		c.Next()
	}
}

// ginLog 日志中间件，记录请求概要信息
func ginLog(logger *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Info(c, fmt.Sprintf(
			"收到新请求 -> client_ip=%s | method=%s | user_agent=%s | path=%s | params=%v",
			c.ClientIP(),
			c.Request.Method,
			c.Request.UserAgent(),
			c.Request.URL.Path,
			c.Request.URL.Query(),
		))
		// 开始计时
		startTime := time.Now()

		// 处理请求
		c.Next()

		// 计算请求处理时间
		duration := time.Since(startTime)

		logger.Info(c, fmt.Sprintf(
			"请求已处理 -> status=%d | duration=%v",
			c.Writer.Status(),
			duration,
		))
	}
}
