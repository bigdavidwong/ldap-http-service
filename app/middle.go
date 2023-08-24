package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"ldap-http-service/lib/ers"
	"ldap-http-service/lib/logger"
	"ldap-http-service/lib/utils"
	"net/http"
	"runtime/debug"
	"time"
)

type ResponseData struct {
	Code    int         `json:"constants"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	TraceId string      `json:"trace_id"`
}

func JsonWithTraceId(c *gin.Context, httpCode, code int, msg string, data interface{}) {
	traceId, _ := c.Value("trace_id").(string)
	response := ResponseData{
		Code:    code,
		Message: msg,
		Data:    data,
		TraceId: traceId,
	}

	// 尝试序列化数据，校验是否有错误
	_, err := json.Marshal(data)
	if err != nil {
		logger.GinLogger.Panicf("unsupported json body -> %v", err)
	}

	c.JSON(httpCode, response)
}

// processRequest 处理请求中间件，包含错误集中处理、panic恢复以及traceId标记等
func processRequest(logger *logrus.Entry) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 在gin自带上下文中标记traceId
		traceID := c.GetHeader("trace_id")
		if traceID == "" {
			// 如果 Header 中不存在 "trace_id"，则自动生成
			traceID = utils.GenUuid("req")
			fmt.Printf("正在设置trace_id.........%s\n", traceID)
		}
		c.Set("trace_id", traceID)

		// panic恢复和异常处理
		defer func() {
			if r := recover(); r != nil {
				logger.Warning(c, fmt.Sprintf("Handler panic! %v", r))
				logger.Debug(c, string(debug.Stack()))

				JsonWithTraceId(c, http.StatusInternalServerError, 1000, "Internal Server Error", map[string]interface{}{})
			}
		}()

		c.Next()
	}
}

func errorHandler(logger *logrus.Entry) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				logger.WithContext(c).Errorf("Handler异常：%v", e)
			}

			// 返回的时候，返回最后一个错误
			var customErr ers.CustomErr

			if errors.As(c.Errors[0], &customErr) {
				JsonWithTraceId(c, customErr.HttpCode(), customErr.Code(), customErr.Error(), nil)
			} else {
				// 其他未知错误
				JsonWithTraceId(c, http.StatusInternalServerError, 1000, "Internal Server Error", map[string]interface{}{})
			}
		}
	}
}

// ginLog 日志中间件，记录请求概要信息
func ginLog(logger *logrus.Entry) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 仅记录健康探活接口以外的api请求信息
		if c.Request.URL.Path != "/ldap/healthz" {
			reqInfo := logrus.Fields{
				"ip":         c.ClientIP(),
				"method":     c.Request.Method,
				"user_agent": c.Request.UserAgent(),
				"path":       c.Request.URL.Path,
				"params":     c.Request.URL.Query(),
			}
			logger.WithContext(c).WithFields(reqInfo).Info("收到http请求")
			// 开始计时
			startTime := time.Now()

			// 处理请求
			c.Next()

			// 计算请求处理时间
			retInfo := logrus.Fields{
				"status":   c.Writer.Status(),
				"duration": time.Since(startTime),
			}
			logger.WithContext(c).WithFields(retInfo).Info("http请求已处理")
		} else {
			c.Next()
		}

	}
}
