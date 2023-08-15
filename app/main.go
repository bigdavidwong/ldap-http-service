package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"ldap-http-service/config"
	"ldap-http-service/lib/logger"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 加载配置
	config.LoadConfig()

	// 启动gin并配置中间件等
	router := gin.New()
	router.Use(processRequest(logger.GinLogger), ginLog(logger.GinLogger), gin.Recovery())

	// 使用自定义的Logger的写入Gin日志
	gin.DefaultWriter = logger.GinLogger.Writer()

	// 设置可信代理IP
	err := router.SetTrustedProxies([]string{"127.0.0.1"})
	if err != nil {
		return
	}

	// 注册路由
	router.GET("/ldap/healthz", handleHealthz)
	router.GET("/ldap/availability", handleCheckAvailability)
	router.GET("/ldap/user/:user_id", handleGetUser)
	router.POST("/ldap/user", handleNewEnableUser)
	router.PATCH("/ldap/user/:user_id", handleUserUpdate)
	router.POST("/ldap/user/:user_id/password", handleUserPwd)
	router.GET("/ldap/group/:group_id", handleGetGroup)
	router.POST("/ldap/group", handleNewGroup)
	router.PATCH("/ldap/group/:group_id", handleGroupUpdate)
	router.PUT("/ldap/group/:group_id/member", handleGroupMember)

	// 启动http服务
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.GinConfig.Listen, config.GinConfig.Port),
		Handler: router,
	}
	go func() {
		if err = srv.ListenAndServe(); err != nil && !errors.Is(http.ErrServerClosed, err) {
			logger.GinLogger.Fatal("异常: http启动失败", err)
		}
	}()

	// 捕获到SIGTERM信号，实现优雅终止
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.GinLogger.Info("Get sigterm signal, shutting down server...")

	// 使用chan启动5秒超时等待，否则强制退出
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 程序退出异常，强行终止
	if err = srv.Shutdown(ctx); err != nil {
		logger.GinLogger.Fatal("Server forced to shutdown: ", err)
	}
	logger.GinLogger.Fatal("Server existed")
}
